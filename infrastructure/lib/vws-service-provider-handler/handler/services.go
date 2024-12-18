package main

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"vws-service-provider-handler/handler/helper"
	"vws-service-provider-handler/handler/internal"
)

// Called when the service was activated in an account. The account details can be found in the internal.Notification object.
// Use-Cases of this notification is a one-time setup of the account in order to be able to consume this service.
func onCreated(ctx context.Context, props *ResourceProps) error {
	log.Print("start registering resource type")

	describeInput := &cloudformation.DescribeTypeInput{
		Type:     "RESOURCE",
		TypeName: aws.String(props.typeName),
	}

	describeResult, err := cfClient.DescribeType(ctx, describeInput)
	if err == nil && describeResult != nil {
		log.Print("resource type already exists: ", *describeResult.TypeName)
		return nil
	}

	input := &cloudformation.RegisterTypeInput{
		Type:                 types.RegistryTypeResource,
		TypeName:             aws.String(props.typeName),
		SchemaHandlerPackage: aws.String(fmt.Sprintf("s3://%s/%s.zip", bucketName, helper.ToKebabCase(props.typeToActivate))),
		ExecutionRoleArn:     aws.String(roleArn),
	}

	log.Printf("try register: %v, %v", *input.TypeName, *input.SchemaHandlerPackage)

	result, err := cfClient.RegisterType(ctx, input)
	if result != nil {
		log.Print("registered successfully")
	}

	return err
}

// This event indicates that an Account Developer requested this service to be removed from their account. Account
// details can be found in the internal.Notification object.
// This notification should trigger a cleanup of resources created in onCreate, which should not persist after this
// service is removed.
//
// Once the cleanup is done, VWS Services expects a confirmation to proceed with deprovisioning the service from the
// consumer's account. You can either call this confirmation URL directly or use the helper method from ConsumerAccounts.
func onRelease(ctx context.Context, props *ResourceProps, n internal.Notification) error {
	log.Print("start releasing resource type")

	describeInput := &cloudformation.DescribeTypeInput{
		Type:     "RESOURCE",
		TypeName: aws.String(props.typeName),
	}

	describeResult, err := cfClient.DescribeType(ctx, describeInput)
	if err != nil && describeResult == nil {
		log.Print("resource type already degregistered")
		return nil
	}

	input := &cloudformation.DeregisterTypeInput{
		Type:     types.RegistryTypeResource,
		TypeName: aws.String(props.typeName),
	}
	result, err := cfClient.DeregisterType(ctx, input)
	if err != nil {
		return err
	}

	log.Print("result: ", result)

	return ConfirmRelease(ctx, n.Confirmation)
}

// An information only notification. Whenever a service is deactivated from a consumer's account, this notification will
// be sent. The notification will be sent _after_ the service was released and fully deprovisioned by VWS Services.
// You may use this event to do further cleanup on _your_ side. You cannot assume any roleArn in the consumer's account
// anymore, as the roles of the service were already been deleted from the consumer's account.
func onDeleted(ctx context.Context, n internal.Notification) error {
	return nil
}

// Indicates that a service was enabled on a project, before it was created in any account.
//
// Requires project notifications enabled for the service.
func onEnabled(ctx context.Context, n internal.Notification) error {
	return nil
}

// Indicates that a service was disabled for a project.
//
// Requires project notifications enabled for the service.
func onDisabled(ctx context.Context, n internal.Notification) error {
	return nil
}

// ConfirmRelease Confirms the release of a service. This will allow VWS Services to continue with the deprovisioning
// process in order to remove the service from the consumer's account.
func ConfirmRelease(ctx context.Context, confirm internal.Confirmation) error {

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
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusAccepted {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("expected accepted Status, got: %d, body: %s", resp.StatusCode, body)
	}
	return err
}
