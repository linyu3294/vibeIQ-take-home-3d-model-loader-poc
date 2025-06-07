package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/apigatewaymanagementapi"
	"github.com/aws/aws-sdk-go/service/dynamodb"
)

type NotificationMessage struct {
	Status  string `json:"status"`
	ModelID string `json:"modelId"`
	Message string `json:"message,omitempty"`
}

func handler(ctx context.Context, sqsEvent events.SQSEvent) error {
	sess := session.Must(session.NewSession())
	dynamo := dynamodb.New(sess)
	api := apigatewaymanagementapi.New(sess, &aws.Config{
		Endpoint: aws.String(os.Getenv("WEBSOCKET_API_ENDPOINT")),
	})

	// Get all active connections
	result, err := dynamo.Scan(&dynamodb.ScanInput{
		TableName: aws.String(os.Getenv("CONNECTIONS_TABLE")),
	})
	if err != nil {
		return fmt.Errorf("failed to scan connections: %v", err)
	}

	// Process each SQS message
	for _, record := range sqsEvent.Records {
		var notification NotificationMessage
		if err := json.Unmarshal([]byte(record.Body), &notification); err != nil {
			fmt.Printf("Failed to unmarshal notification: %v\n", err)
			continue
		}

		// Send to all active connections
		for _, item := range result.Items {
			connectionID := *item["connectionId"].S
			data, _ := json.Marshal(notification)

			_, err := api.PostToConnection(&apigatewaymanagementapi.PostToConnectionInput{
				ConnectionId: aws.String(connectionID),
				Data:         data,
			})
			if err != nil {
				fmt.Printf("Failed to send to connection %s: %v\n", connectionID, err)
				// Don't return error, continue with other connections
			}
		}
	}

	return nil
}

func main() {
	lambda.Start(handler)
}
