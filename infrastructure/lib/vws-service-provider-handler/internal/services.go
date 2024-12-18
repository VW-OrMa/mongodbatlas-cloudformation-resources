package internal

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"vws-service-provider-handler/internal/helper"
)

type Service struct {
	cfClient   *cloudformation.Client
	roleArn    string
	bucketName string
}

func NewService(cfClient *cloudformation.Client, roleArn string, bucketName string) *Service {
	return &Service{
		cfClient:   cfClient,
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
		Type:                 types.RegistryTypeResource,
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

	describeInput := &cloudformation.DescribeTypeInput{
		Type:     "RESOURCE",
		TypeName: aws.String(props.TypeName),
	}

	describeResult, err := s.cfClient.DescribeType(ctx, describeInput)
	if err != nil && describeResult == nil {
		log.Print("resource type already deregistered")
		return nil
	}

	input := &cloudformation.DeregisterTypeInput{
		Type:     types.RegistryTypeResource,
		TypeName: aws.String(props.TypeName),
	}
	_, err = s.cfClient.DeregisterType(ctx, input)
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

// Confirms the release of a service. This will allow VWS Service to continue with the deprovisioning
// process in order to remove the service from the consumer's account.
func confirmRelease(ctx context.Context, confirm Confirmation) error {
	log.Print("confirming release")

	httpClient := http.Client{
		Transport: &http.Transport{
			Proxy: func(r *http.Request) (*url.URL, error) {
				if strings.HasSuffix(r.URL.Host, "admin.vwapps.cloud") {
					return url.Parse(os.Getenv("SERVICES_PROXY"))
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
