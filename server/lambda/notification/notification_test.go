package main

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/assert"
)

type mockDynamoDBClient struct {
	getItemInput  *dynamodb.GetItemInput
	getItemOutput *dynamodb.GetItemOutput
	getItemErr    error

	putItemInput  *dynamodb.PutItemInput
	putItemOutput *dynamodb.PutItemOutput
	putItemErr    error

	queryInput  *dynamodb.QueryInput
	queryOutput *dynamodb.QueryOutput
	queryErr    error
}

func (m *mockDynamoDBClient) GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	m.getItemInput = params
	return m.getItemOutput, m.getItemErr
}

func (m *mockDynamoDBClient) PutItem(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	m.putItemInput = params
	return m.putItemOutput, m.putItemErr
}

func (m *mockDynamoDBClient) Query(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
	m.queryInput = params
	return m.queryOutput, m.queryErr
}

type mockAPIGatewayClient struct {
	postToConnectionInput  *apigatewaymanagementapi.PostToConnectionInput
	postToConnectionOutput *apigatewaymanagementapi.PostToConnectionOutput
	postToConnectionErr    error
}

func (m *mockAPIGatewayClient) PostToConnection(ctx context.Context, params *apigatewaymanagementapi.PostToConnectionInput, optFns ...func(*apigatewaymanagementapi.Options)) (*apigatewaymanagementapi.PostToConnectionOutput, error) {
	m.postToConnectionInput = params
	return m.postToConnectionOutput, m.postToConnectionErr
}

func setupTestEnv(t *testing.T) func() {
	// Set environment variables
	os.Setenv("connections_table", "test-connections-table")
	os.Setenv("job_history_table", "test-job-history-table")
	os.Setenv("websocket_api_endpoint", "https://test-api.execute-api.us-east-1.amazonaws.com/test")

	// Return cleanup function
	return func() {
		os.Unsetenv("connections_table")
		os.Unsetenv("job_history_table")
		os.Unsetenv("websocket_api_endpoint")
	}
}

func TestHandler_SuccessfulMessageProcessing(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	notification := NotificationMessage{
		ConnectionID: "test-connection-id",
		JobType:      "conversion",
		JobID:        "test-job-id",
		JobStatus:    "completed",
		FromFileType: "blend",
		ToFileType:   "glb",
		ModelID:      "test-model-id",
		S3Key:        "test-s3-key",
		NewS3Key:     "test-new-s3-key",
	}
	notificationBody, _ := json.Marshal(notification)

	mockDynamo := &mockDynamoDBClient{
		getItemOutput: &dynamodb.GetItemOutput{
			Item: map[string]types.AttributeValue{
				"connectionId": &types.AttributeValueMemberS{Value: "test-connection-id"},
			},
		},
		queryOutput: &dynamodb.QueryOutput{
			Items: []map[string]types.AttributeValue{},
		},
	}

	mockAPI := &mockAPIGatewayClient{}

	// Create test event
	event := events.SQSEvent{
		Records: []events.SQSMessage{
			{
				Body: string(notificationBody),
			},
		},
	}

	err := HandlerWithClients(context.Background(), event, mockDynamo, mockAPI)

	assert.NoError(t, err)
	assert.NotNil(t, mockDynamo.getItemInput)
	assert.Equal(t, "test-connection-id", mockDynamo.getItemInput.Key["connectionId"].(*types.AttributeValueMemberS).Value)
	assert.NotNil(t, mockDynamo.putItemInput)
	assert.Equal(t, "test-job-id", mockDynamo.putItemInput.Item["jobId"].(*types.AttributeValueMemberS).Value)
	assert.NotNil(t, mockAPI.postToConnectionInput)
	assert.Equal(t, "test-connection-id", *mockAPI.postToConnectionInput.ConnectionId)
	assert.Equal(t, notificationBody, mockAPI.postToConnectionInput.Data)
}

func TestHandler_ErrorHandling(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	notification := NotificationMessage{
		ConnectionID: "test-connection-id",
		JobType:      "conversion",
		JobID:        "test-job-id",
		JobStatus:    "failed",
		FromFileType: "blend",
		ToFileType:   "glb",
		ModelID:      "test-model-id",
		S3Key:        "test-s3-key",
		Error:        "test error",
	}
	notificationBody, _ := json.Marshal(notification)

	mockDynamo := &mockDynamoDBClient{
		getItemOutput: &dynamodb.GetItemOutput{
			Item: map[string]types.AttributeValue{
				"connectionId": &types.AttributeValueMemberS{Value: "test-connection-id"},
			},
		},
		queryOutput: &dynamodb.QueryOutput{
			Items: []map[string]types.AttributeValue{},
		},
	}

	mockAPI := &mockAPIGatewayClient{}

	event := events.SQSEvent{
		Records: []events.SQSMessage{
			{
				Body: string(notificationBody),
			},
		},
	}

	err := HandlerWithClients(context.Background(), event, mockDynamo, mockAPI)

	assert.NoError(t, err)
	assert.NotNil(t, mockDynamo.putItemInput)
	assert.Equal(t, "failed", mockDynamo.putItemInput.Item["jobStatus"].(*types.AttributeValueMemberS).Value)
	assert.Equal(t, "test error", mockDynamo.putItemInput.Item["error"].(*types.AttributeValueMemberS).Value)
	assert.NotNil(t, mockAPI.postToConnectionInput)
	assert.Equal(t, notificationBody, mockAPI.postToConnectionInput.Data)
}

func TestHandler_JobHistorySaving(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	notification := NotificationMessage{
		ConnectionID: "test-connection-id",
		JobType:      "conversion",
		JobID:        "test-job-id",
		JobStatus:    "completed",
		FromFileType: "blend",
		ToFileType:   "glb",
		ModelID:      "test-model-id",
		S3Key:        "test-s3-key",
		NewS3Key:     "test-new-s3-key",
	}
	notificationBody, _ := json.Marshal(notification)

	mockDynamo := &mockDynamoDBClient{
		getItemOutput: &dynamodb.GetItemOutput{
			Item: map[string]types.AttributeValue{
				"connectionId": &types.AttributeValueMemberS{Value: "test-connection-id"},
			},
		},
		queryOutput: &dynamodb.QueryOutput{
			Items: []map[string]types.AttributeValue{},
		},
	}

	mockAPI := &mockAPIGatewayClient{}

	event := events.SQSEvent{
		Records: []events.SQSMessage{
			{
				Body: string(notificationBody),
			},
		},
	}

	err := HandlerWithClients(context.Background(), event, mockDynamo, mockAPI)

	assert.NoError(t, err)
	assert.NotNil(t, mockDynamo.putItemInput)

	// Verify all fields are saved correctly
	assert.Equal(t, "test-job-id", mockDynamo.putItemInput.Item["jobId"].(*types.AttributeValueMemberS).Value)
	assert.Equal(t, "test-connection-id", mockDynamo.putItemInput.Item["connectionId"].(*types.AttributeValueMemberS).Value)
	assert.Equal(t, "conversion", mockDynamo.putItemInput.Item["jobType"].(*types.AttributeValueMemberS).Value)
	assert.Equal(t, "completed", mockDynamo.putItemInput.Item["jobStatus"].(*types.AttributeValueMemberS).Value)
	assert.Equal(t, "blend", mockDynamo.putItemInput.Item["fromFileType"].(*types.AttributeValueMemberS).Value)
	assert.Equal(t, "glb", mockDynamo.putItemInput.Item["toFileType"].(*types.AttributeValueMemberS).Value)
	assert.Equal(t, "test-model-id", mockDynamo.putItemInput.Item["modelId"].(*types.AttributeValueMemberS).Value)
	assert.Equal(t, "test-s3-key", mockDynamo.putItemInput.Item["s3Key"].(*types.AttributeValueMemberS).Value)
	assert.Equal(t, "test-new-s3-key", mockDynamo.putItemInput.Item["newS3Key"].(*types.AttributeValueMemberS).Value)
	assert.NotEmpty(t, mockDynamo.putItemInput.Item["timestamp"].(*types.AttributeValueMemberS).Value)
}
