package internal

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cfTypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"vws-service-provider-handler/internal/helper"
)

var (
	region          = envOrFail("AWS_REGION")
	roleArnTemplate = envOrFail("EXECUTION_ROLE_ARN_TEMPLATE")
	servicesProxy   = envOrFail("SERVICES_PROXY")
	bucketName      = envOrFail("BUCKET_NAME")
)

type Service struct {
	cfClient   *cloudformation.Client
	ec2Client  *ec2.Client
	roleArn    string
	bucketName string
}

func NewService(ctx context.Context, accountId string) *Service {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		log.Fatalf("Failed to load AWS SDK config: %v", err)
	}

	roleArn := strings.Replace(roleArnTemplate, "{ACCOUNT_ID}", accountId, 1)

	// Get service role credentials to handle actions on other accounts
	stsClient := sts.NewFromConfig(cfg)
	credentials := stscreds.NewAssumeRoleProvider(stsClient, roleArn)

	// To create the VPC endpoint in the target account (TODO)
	ec2Client := ec2.NewFromConfig(cfg, func(options *ec2.Options) {
		options.Credentials = credentials
	})

	cfClient := cloudformation.NewFromConfig(cfg, func(options *cloudformation.Options) {
		options.Credentials = credentials
	})

	return &Service{
		cfClient:   cfClient,
		ec2Client:  ec2Client,
		roleArn:    roleArn,
		bucketName: bucketName,
	}
}

type ResourceProps struct {
	TypeName       string
	TypeToActivate string
}

// OnCreated Called when the service was activated in an account. The account details can be found in the internal.Notification object.
// Use-Cases of this notification is a one-time setup of the account in order to be able to consume this service.
func (s *Service) OnCreated(ctx context.Context, props *ResourceProps) error {
	log.Print("start registering resource type")

	describeInput := &cloudformation.DescribeTypeInput{
		Type:     "RESOURCE",
		TypeName: aws.String(props.TypeName),
	}

	describeResult, err := s.cfClient.DescribeType(ctx, describeInput)
	if err == nil && describeResult != nil {
		log.Print("resource type already exists: ", *describeResult.TypeName)
		return nil
	}

	input := &cloudformation.RegisterTypeInput{
		Type:                 cfTypes.RegistryTypeResource,
		TypeName:             aws.String(props.TypeName),
		SchemaHandlerPackage: aws.String(fmt.Sprintf("s3://%s/%s.zip", s.bucketName, helper.ToKebabCase(props.TypeToActivate))),
		ExecutionRoleArn:     aws.String(s.roleArn),
	}

	log.Printf("try register: %v, %v", *input.TypeName, *input.SchemaHandlerPackage)

	result, err := s.cfClient.RegisterType(ctx, input)
	if result != nil {
		log.Print("registered successfully")
	}

	return err
}

// OnRelease This event indicates that an Account Developer requested this service to be removed from their account. Account
// details can be found in the internal.Notification object.
// This notification should trigger a cleanup of resources created in onCreate, which should not persist after this
// service is removed.
//
// Once the cleanup is done, VWS Service expects a confirmation to proceed with deprovisioning the service from the
// consumer's account. You can either call this confirmation URL directly or use the helper method from ConsumerAccounts.
func (s *Service) OnRelease(ctx context.Context, props *ResourceProps, n Notification) error {
	log.Print("start releasing resource type")

	registryType := cfTypes.RegistryTypeResource
	typeName := aws.String(props.TypeName)

	describeInput := &cloudformation.DescribeTypeInput{
		Type:     registryType,
		TypeName: typeName,
	}
	describeResult, err := s.cfClient.DescribeType(ctx, describeInput)
	if err != nil {
		var apiErr *cfTypes.InvalidOperationException
		if ok := errors.As(err, &apiErr); ok {
			log.Print("Type is already deregistered or does not exist: ", apiErr.Message)
			return nil
		}
		log.Fatalf("failed to describe type: %v", err)
	}
	if describeResult.DeprecatedStatus == cfTypes.DeprecatedStatusDeprecated {
		fmt.Printf("Type is already deprecated and fully deregistered")
		return nil
	}

	// Must be done before a type can be deregistered completely
	s.deregisterAllTypeVersions(ctx, registryType, typeName)

	_, err = s.cfClient.DeregisterType(ctx, &cloudformation.DeregisterTypeInput{
		Type:     registryType,
		TypeName: typeName,
	})
	if err == nil {
		log.Print("deregistered successfully")
	}

	return confirmRelease(ctx, n.Confirmation)
}

// OnDeleted An information only notification. Whenever a service is deactivated from a consumer's account, this notification will
// be sent. The notification will be sent _after_ the service was released and fully deprovisioned by VWS Service.
// You may use this event to do further cleanup on _your_ side. You cannot assume any roleArn in the consumer's account
// anymore, as the roles of the service were already been deleted from the consumer's account.
func (s *Service) OnDeleted(ctx context.Context, n Notification) error {
	return nil
}

// OnEnabled Indicates that a service was enabled on a project, before it was created in any account.
//
// Requires project notifications enabled for the service.
func (s *Service) OnEnabled(ctx context.Context, n Notification) error {
	return nil
}

// OnDisabled // Indicates that a service was disabled for a project.
// //
// // Requires project notifications enabled for the service.
func (s *Service) OnDisabled(ctx context.Context, n Notification) error {
	return nil
}

func (s *Service) deregisterAllTypeVersions(ctx context.Context, registryType cfTypes.RegistryType, typeName *string) {
	versions, err := s.cfClient.ListTypeVersions(context.TODO(), &cloudformation.ListTypeVersionsInput{
		Type:     registryType,
		TypeName: typeName,
	})
	if err != nil {
		log.Fatalf("failed to list type versions: %v", err)
	}
	for _, version := range versions.TypeVersionSummaries {
		if version.IsDefaultVersion != nil && *version.IsDefaultVersion {
			fmt.Printf("Skipping deregistration of default version: %s\n", aws.ToString(version.Arn))
			continue
		}

		_, err := s.cfClient.DeregisterType(ctx, &cloudformation.DeregisterTypeInput{
			Arn: version.Arn,
		})
		if err != nil {
			log.Printf("failed to deregister version %s: %v", aws.ToString(version.Arn), err)
		}
	}
}

// Confirms the release of a service. This will allow VWS Service to continue with the deprovisioning
// process in order to remove the service from the consumer's account.
func confirmRelease(ctx context.Context, confirm Confirmation) error {
	log.Print("confirming release")

	httpClient := http.Client{
		Transport: &http.Transport{
			Proxy: func(r *http.Request) (*url.URL, error) {
				if strings.HasSuffix(r.URL.Host, "admin.vwapps.cloud") {
					return url.Parse(servicesProxy)
				}
				return nil, nil
			}},
	}

	r, err := confirm.Request()
	if err != nil {
		return err
	}
	r.WithContext(ctx)
	resp, err := httpClient.Do(r)
	if err != nil {
		log.Print("error confirming release at vws. Deadline for automatic confirm is set to: ", confirm.Deadline)
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("expected accepted Status, got: %d, body: %s", resp.StatusCode, body)
	}
	return err
}

func envOrFail(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic("missing configuration environment entry: " + key)
	}
	return v
}
