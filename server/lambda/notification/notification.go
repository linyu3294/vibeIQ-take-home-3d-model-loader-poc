package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
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

type DynamoDBClient interface {
	GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	PutItem(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	Query(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
}

type APIGatewayClient interface {
	PostToConnection(ctx context.Context, params *apigatewaymanagementapi.PostToConnectionInput, optFns ...func(*apigatewaymanagementapi.Options)) (*apigatewaymanagementapi.PostToConnectionOutput, error)
}

func HandlerWithClients(ctx context.Context, sqsEvent events.SQSEvent, dynamoClient DynamoDBClient, apiClient APIGatewayClient) error {
	connectionsTable := os.Getenv("connections_table")
	jobHistoryTable := os.Getenv("job_history_table")
	websocketEndpoint := os.Getenv("websocket_api_endpoint")

	log.Printf("connectionsTable: '%s' (len=%d)", connectionsTable, len(connectionsTable))
	log.Printf("notificationsTable: '%s'", jobHistoryTable)
	log.Printf("websocket_api_endpoint: '%s'", websocketEndpoint)

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

		// Check for existing record with same modelId, jobType, fromFileType, and toFileType
		queryInput := &dynamodb.QueryInput{
			TableName:              &jobHistoryTable,
			IndexName:              aws.String("ModelJobTypeIndex"),
			KeyConditionExpression: aws.String("modelId = :modelId AND jobType = :jobType"),
			FilterExpression:       aws.String("fromFileType = :fromFileType AND toFileType = :toFileType"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":modelId":      &types.AttributeValueMemberS{Value: notification.ModelID},
				":jobType":      &types.AttributeValueMemberS{Value: notification.JobType},
				":fromFileType": &types.AttributeValueMemberS{Value: notification.FromFileType},
				":toFileType":   &types.AttributeValueMemberS{Value: notification.ToFileType},
			},
		}

		queryResult, err := dynamoClient.Query(ctx, queryInput)
		if err != nil {
			log.Printf("Error querying for existing record: %v", err)
			continue
		}

		var existingJobId string
		if len(queryResult.Items) > 0 {
			// Found existing record, use its jobId
			existingJobId = queryResult.Items[0]["jobId"].(*types.AttributeValueMemberS).Value
		} else {
			// No existing record, use the new jobId
			existingJobId = notification.JobID
		}

		putInput := &dynamodb.PutItemInput{
			TableName: &jobHistoryTable,
			Item: map[string]types.AttributeValue{
				"jobId":        &types.AttributeValueMemberS{Value: existingJobId},
				"connectionId": &types.AttributeValueMemberS{Value: notification.ConnectionID},
				"jobType":      &types.AttributeValueMemberS{Value: notification.JobType},
				"jobStatus":    &types.AttributeValueMemberS{Value: notification.JobStatus},
				"fromFileType": &types.AttributeValueMemberS{Value: notification.FromFileType},
				"toFileType":   &types.AttributeValueMemberS{Value: notification.ToFileType},
				"modelId":      &types.AttributeValueMemberS{Value: notification.ModelID},
				"s3Key":        &types.AttributeValueMemberS{Value: notification.S3Key},
				"newS3Key":     &types.AttributeValueMemberS{Value: notification.NewS3Key},
				"error":        &types.AttributeValueMemberS{Value: notification.Error},
				"timestamp":    &types.AttributeValueMemberS{Value: time.Now().UTC().Format(time.RFC3339)},
			},
		}

		_, err = dynamoClient.PutItem(ctx, putInput)
		if err != nil {
			log.Printf("Error saving notification to DynamoDB: %v", err)
			continue
		}

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

func handler(ctx context.Context, sqsEvent events.SQSEvent) error {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("unable to load SDK config: %v", err)
	}
	dynamoClient := dynamodb.NewFromConfig(cfg)
	websocketEndpoint := os.Getenv("websocket_api_endpoint")

	apiEndpoint := websocketEndpoint
	apiClient := apigatewaymanagementapi.NewFromConfig(cfg, func(o *apigatewaymanagementapi.Options) {
		o.BaseEndpoint = &apiEndpoint
	})
	return HandlerWithClients(ctx, sqsEvent, dynamoClient, apiClient)
}

func main() {
	lambda.Start(handler)
}
