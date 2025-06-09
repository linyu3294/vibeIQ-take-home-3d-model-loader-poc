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
	Status      string `json:"status"`
	ModelID     string `json:"modelId"`
	OutputS3Key string `json:"outputS3Key,omitempty"`
	Error       string `json:"error,omitempty"`
}

func handler(ctx context.Context, sqsEvent events.SQSEvent) error {
	// Load AWS configuration
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("unable to load SDK config: %v", err)
	}

	// Initialize DynamoDB client
	dynamoClient := dynamodb.NewFromConfig(cfg)

	// Get environment variables
	connectionsTable := os.Getenv("CONNECTIONS_TABLE")
	websocketEndpoint := os.Getenv("WEBSOCKET_API_ENDPOINT")

	// Initialize API Gateway client
	apiClient := apigatewaymanagementapi.NewFromConfig(cfg, func(o *apigatewaymanagementapi.Options) {
		o.BaseEndpoint = &websocketEndpoint
	})

	// Process each message from SQS
	for _, record := range sqsEvent.Records {
		// Parse notification message
		var notification NotificationMessage
		if err := json.Unmarshal([]byte(record.Body), &notification); err != nil {
			log.Printf("Error unmarshaling notification: %v", err)
			continue
		}

		// Scan DynamoDB for all connections
		scanInput := &dynamodb.ScanInput{
			TableName: &connectionsTable,
		}

		result, err := dynamoClient.Scan(ctx, scanInput)
		if err != nil {
			return fmt.Errorf("error scanning connections table: %v", err)
		}

		// Send notification to all connected clients
		for _, item := range result.Items {
			connectionID := item["connectionId"].(*types.AttributeValueMemberS).Value

			// Send message to WebSocket connection
			_, err := apiClient.PostToConnection(ctx, &apigatewaymanagementapi.PostToConnectionInput{
				ConnectionId: &connectionID,
				Data:         []byte(record.Body),
			})

			if err != nil {
				log.Printf("Error sending message to connection %s: %v", connectionID, err)
				// Continue with other connections even if one fails
				continue
			}
		}
	}

	return nil
}

func main() {
	lambda.Start(handler)
}
