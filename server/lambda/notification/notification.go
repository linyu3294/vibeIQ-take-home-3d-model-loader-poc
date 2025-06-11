package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type NotificationMessage struct {
	ConnectionID string `json:"connectionId"`
	JobType      string `json:"jobType"`
	JobID        string `json:"jobId"`
	JobStatus    string `json:"jobStatus"`
	FromFileType string `json:"fromFileType"`
	ToFileType   string `json:"toFileType"`
	ModelID      string `json:"modelId"`
	S3Key        string `json:"s3Key"`
	NewS3Key     string `json:"newS3Key"`
	Error        string `json:"error"`
}

func handler(ctx context.Context, sqsEvent events.SQSEvent) error {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("unable to load SDK config: %v", err)
	}

	dynamoClient := dynamodb.NewFromConfig(cfg)
	connectionsTable := os.Getenv("connections_table")
	websocketEndpoint := os.Getenv("websocket_api_endpoint")

	log.Printf("connectionsTable: '%s' (len=%d)", connectionsTable, len(connectionsTable))
	log.Printf("websocket_api_endpoint: '%s'", websocketEndpoint)

	apiClient := apigatewaymanagementapi.NewFromConfig(cfg, func(o *apigatewaymanagementapi.Options) {
		o.BaseEndpoint = &websocketEndpoint
	})

	for _, record := range sqsEvent.Records {
		var notification NotificationMessage
		if err := json.Unmarshal([]byte(record.Body), &notification); err != nil {
			log.Printf("Error unmarshaling notification: %v", err)
			continue
		}

		if notification.ConnectionID == "" {
			log.Printf("No connectionId in notification message, skipping.")
			continue
		}

		// Get the connection from DynamoDB
		getInput := &dynamodb.GetItemInput{
			TableName: &connectionsTable,
			Key: map[string]types.AttributeValue{
				"connectionId": &types.AttributeValueMemberS{Value: notification.ConnectionID},
			},
		}
		getResult, err := dynamoClient.GetItem(ctx, getInput)
		if err != nil {
			log.Printf("Error getting connectionId %s: %v", notification.ConnectionID, err)
			continue
		}
		if getResult.Item == nil {
			log.Printf("connectionId %s not found in DynamoDB, skipping.", notification.ConnectionID)
			continue
		}

		// Relay the full SQS message body to the WebSocket client
		_, err = apiClient.PostToConnection(ctx, &apigatewaymanagementapi.PostToConnectionInput{
			ConnectionId: &notification.ConnectionID,
			Data:         []byte(record.Body),
		})
		if err != nil {
			log.Printf("Error sending message to connection %s: %v", notification.ConnectionID, err)
			continue
		}
		log.Printf("Notification relayed to connectionId %s", notification.ConnectionID)
	}

	return nil
}

func main() {
	lambda.Start(handler)
}
