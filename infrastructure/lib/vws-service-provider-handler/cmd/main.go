package main // this MUST be called "main" for lambda entrypoint

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"log"
	"os"
	"strings"
	"vws-service-provider-handler/internal"
)

var (
	cfClient *cloudformation.Client
)

func HandleVwsProviderNotification(ctx context.Context, sqsEvent events.SQSEvent) error {

	typesToActivate := os.Getenv("TYPES_TO_ACTIVATE")
	log.Print("typesToActivate: ", typesToActivate)

	typesToActivateList := strings.Split(typesToActivate, ",")
	if len(typesToActivateList) == 0 {
		log.Println("TYPES_TO_ACTIVATE is not set")
		return errors.New("TYPES_TO_ACTIVATE is not set")
	}

	bucketName := os.Getenv("BUCKET_NAME")
	if bucketName == "" {
		log.Println("BUCKET_NAME is not set")
		return errors.New("BUCKET_NAME is not set")
	}

	roleArn := os.Getenv("EXECUTION_ROLE_ARN")
	if roleArn == "" {
		log.Println("EXECUTION_ROLE_ARN is not set")
		return errors.New("EXECUTION_ROLE_ARN is not set")
	}

	notification, err := internal.ReadNotification(sqsEvent.Records[0].Body)
	if err != nil {
		log.Print(fmt.Errorf("unable to load config: %v", err))
		return err
	}

	service := internal.NewService(cfClient, roleArn, bucketName)

	for _, typeToActivate := range typesToActivateList {
		typeName := "MongoDB::Atlas::" + typeToActivate
		log.SetPrefix(fmt.Sprintf("[%s] ", typeName))

		props := &internal.ResourceProps{
			TypeName:       typeName,
			TypeToActivate: typeToActivate,
		}

		switch notification.Action {
		case internal.ActionEnabled:
			err = service.OnEnabled(ctx, notification)
			break
		case internal.ActionDisabled:
			err = service.OnDisabled(ctx, notification)
			break
		case internal.ActionCreated:
			err = service.OnCreated(ctx, props)
			break
		case internal.ActionRelease:
			err = service.OnRelease(ctx, props, notification)
			break
		case internal.ActionDeleted:
			err = service.OnDeleted(ctx, notification)
		default:
			log.Print(fmt.Errorf("unknown action: %s", notification.Action))
			return nil
		}
		if err != nil {
			log.Print(fmt.Errorf("error handling: %s", err))
			return err
		}
	}
	return nil
}

// Include any code you want Lambda to run during the initialization phase
func init() {
	ctx := context.TODO()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("Failed to load AWS SDK config: %v", err)
	}

	cfClient = cloudformation.NewFromConfig(cfg)
}

// This is a required entry point for your Lambda handler. The argument to the lambda.Start() method is your main handler method.
func main() {
	lambda.Start(HandleVwsProviderNotification)
}
