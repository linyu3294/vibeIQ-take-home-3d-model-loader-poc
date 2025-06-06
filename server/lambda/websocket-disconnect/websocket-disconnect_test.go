package main

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/stretchr/testify/assert"
)

type mockDynamoDB struct {
	dynamodbiface.DynamoDBAPI
	deleteItemErr   error
	deleteItemInput *dynamodb.DeleteItemInput
}

func (m *mockDynamoDB) DeleteItem(input *dynamodb.DeleteItemInput) (*dynamodb.DeleteItemOutput, error) {
	m.deleteItemInput = input
	return &dynamodb.DeleteItemOutput{}, m.deleteItemErr
}

func TestHandleDisconnect_ValidAPIKey(t *testing.T) {
	os.Setenv("api_key_value", "test-api-key")
	defer os.Unsetenv("api_key_value")

	mockDynamo := &mockDynamoDB{}
	req := events.APIGatewayWebsocketProxyRequest{
		RequestContext: events.APIGatewayWebsocketProxyRequestContext{
			ConnectionID: "test-connection-id",
		},
		QueryStringParameters: map[string]string{
			"apiKey": "test-api-key",
		},
	}
	resp, err := HandleDisconnect(context.Background(), req, mockDynamo, "test-table")
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "Disconnected", resp.Body)
}

func TestHandleDisconnect_MissingAPIKey(t *testing.T) {
	mockDynamo := &mockDynamoDB{}
	req := events.APIGatewayWebsocketProxyRequest{
		RequestContext: events.APIGatewayWebsocketProxyRequestContext{
			ConnectionID: "test-connection-id",
		},
		QueryStringParameters: map[string]string{},
	}
	resp, err := HandleDisconnect(context.Background(), req, mockDynamo, "test-table")
	assert.NoError(t, err)
	assert.Equal(t, 401, resp.StatusCode)
	assert.Contains(t, resp.Body, "API key is required")
}

func TestHandleDisconnect_InvalidAPIKey(t *testing.T) {
	os.Setenv("api_key_value", "test-api-key")
	defer os.Unsetenv("api_key_value")

	mockDynamo := &mockDynamoDB{}
	req := events.APIGatewayWebsocketProxyRequest{
		RequestContext: events.APIGatewayWebsocketProxyRequestContext{
			ConnectionID: "test-connection-id",
		},
		QueryStringParameters: map[string]string{
			"apiKey": "wrong-api-key",
		},
	}
	resp, err := HandleDisconnect(context.Background(), req, mockDynamo, "test-table")
	assert.NoError(t, err)
	assert.Equal(t, 403, resp.StatusCode)
	assert.Contains(t, resp.Body, "Invalid API key")
}

func TestHandleDisconnect_Success(t *testing.T) {
	mockDynamo := &mockDynamoDB{}
	req := events.APIGatewayWebsocketProxyRequest{
		RequestContext: events.APIGatewayWebsocketProxyRequestContext{
			ConnectionID: "test-connection-id",
		},
	}
	resp, err := HandleDisconnect(context.Background(), req, mockDynamo, "test-table")
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "Disconnected", resp.Body)
	assert.NotNil(t, mockDynamo.deleteItemInput)
	assert.Equal(t, aws.String("test-table"), mockDynamo.deleteItemInput.TableName)
	assert.Equal(t, "test-connection-id", *mockDynamo.deleteItemInput.Key["connectionId"].S)
}

func TestHandleDisconnect_MissingTable(t *testing.T) {
	mockDynamo := &mockDynamoDB{}
	req := events.APIGatewayWebsocketProxyRequest{
		RequestContext: events.APIGatewayWebsocketProxyRequestContext{
			ConnectionID: "test-connection-id",
		},
	}
	resp, err := HandleDisconnect(context.Background(), req, mockDynamo, "")
	assert.NoError(t, err)
	assert.Equal(t, 500, resp.StatusCode)
	assert.Equal(t, "CONNECTIONS_TABLE not set", resp.Body)
}

func TestHandleDisconnect_DynamoError(t *testing.T) {
	mockDynamo := &mockDynamoDB{deleteItemErr: errors.New("dynamo error")}
	req := events.APIGatewayWebsocketProxyRequest{
		RequestContext: events.APIGatewayWebsocketProxyRequestContext{
			ConnectionID: "test-connection-id",
		},
	}
	resp, err := HandleDisconnect(context.Background(), req, mockDynamo, "test-table")
	assert.NoError(t, err)
	assert.Equal(t, 500, resp.StatusCode)
	assert.Equal(t, "dynamo error", resp.Body)
}
