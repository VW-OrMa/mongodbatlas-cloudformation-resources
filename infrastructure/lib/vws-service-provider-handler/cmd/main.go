package main // this MUST be called "main" for lambda entrypoint

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"log"
	"os"
	"strings"
	"vws-service-provider-handler/internal"
)

func HandleVwsProviderNotification(ctx context.Context, sqsEvent events.SQSEvent) error {

	notification, err := internal.ReadNotification(sqsEvent.Records[0].Body)
	if err != nil {
		log.Print(fmt.Errorf("unable to read config: %v", err))
		return err
	}

	typesToActivate := os.Getenv("TYPES_TO_ACTIVATE")
	log.Print("typesToActivate: ", typesToActivate)

	typesToActivateList := strings.Split(typesToActivate, ",")
	if len(typesToActivateList) == 0 {
		log.Println("TYPES_TO_ACTIVATE is not set")
		return errors.New("TYPES_TO_ACTIVATE is not set")
	}

	service := internal.NewService(ctx, notification.AccountId)

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

// This is a required entry point for your Lambda handler. The argument to the lambda.Start() method is your main handler method.
func main() {
	lambda.Start(HandleVwsProviderNotification)
}
