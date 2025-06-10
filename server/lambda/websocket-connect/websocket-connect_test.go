package main

import (
	"context"
	"errors"
	"log"
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
	putItemErr   error
	putItemInput *dynamodb.PutItemInput
}

func (m *mockDynamoDB) PutItem(input *dynamodb.PutItemInput) (*dynamodb.PutItemOutput, error) {
	m.putItemInput = input
	return &dynamodb.PutItemOutput{}, m.putItemErr
}

func TestHandleConnect_ValidAPIKey(t *testing.T) {
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
	resp, err := HandleConnect(context.Background(), req, mockDynamo, "test-table")
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "Connected", resp.Body)
}

func TestHandleConnect_MissingAPIKey(t *testing.T) {
	mockDynamo := &mockDynamoDB{}
	req := events.APIGatewayWebsocketProxyRequest{
		RequestContext: events.APIGatewayWebsocketProxyRequestContext{
			ConnectionID: "test-connection-id",
		},
		QueryStringParameters: map[string]string{},
	}
	resp, err := HandleConnect(context.Background(), req, mockDynamo, "test-table")
	assert.NoError(t, err)
	assert.Equal(t, 401, resp.StatusCode)
	assert.Contains(t, resp.Body, "API key is required")
}

func TestHandleConnect_InvalidAPIKey(t *testing.T) {
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
	resp, err := HandleConnect(context.Background(), req, mockDynamo, "test-table")
	assert.NoError(t, err)
	assert.Equal(t, 403, resp.StatusCode)
	assert.Contains(t, resp.Body, "Invalid API key")
}

func TestHandleConnect_Success(t *testing.T) {
	os.Setenv("api_key_value", "test-api-key")
	mockDynamo := &mockDynamoDB{}
	req := events.APIGatewayWebsocketProxyRequest{
		RequestContext: events.APIGatewayWebsocketProxyRequestContext{
			ConnectionID: "test-connection-id",
		},
		QueryStringParameters: map[string]string{
			"apiKey": "test-api-key",
		},
	}
	resp, err := HandleConnect(context.Background(), req, mockDynamo, "test-table")
	assert.NoError(t, err)
	log.Printf("resp: %+v\n", resp)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "Connected", resp.Body)
	assert.NotNil(t, mockDynamo.putItemInput)
	assert.Equal(t, aws.String("test-table"), mockDynamo.putItemInput.TableName)
	assert.Equal(t, "test-connection-id", *mockDynamo.putItemInput.Item["connectionId"].S)
}

func TestHandleConnect_MissingTable(t *testing.T) {
	os.Setenv("api_key_value", "test-api-key")
	mockDynamo := &mockDynamoDB{}
	req := events.APIGatewayWebsocketProxyRequest{
		RequestContext: events.APIGatewayWebsocketProxyRequestContext{
			ConnectionID: "test-connection-id",
		},
		QueryStringParameters: map[string]string{
			"apiKey": "test-api-key",
		},
	}
	resp, err := HandleConnect(context.Background(), req, mockDynamo, "")
	assert.NoError(t, err)
	log.Printf("resp: %+v\n", resp)
	assert.Equal(t, 500, resp.StatusCode)
	assert.Equal(t, "connections_table not set", resp.Body)
}

func TestHandleConnect_DynamoError(t *testing.T) {
	os.Setenv("api_key_value", "test-api-key")
	mockDynamo := &mockDynamoDB{putItemErr: errors.New("dynamo error")}
	req := events.APIGatewayWebsocketProxyRequest{
		RequestContext: events.APIGatewayWebsocketProxyRequestContext{
			ConnectionID: "test-connection-id",
		},
		QueryStringParameters: map[string]string{
			"apiKey": "test-api-key",
		},
	}
	resp, err := HandleConnect(context.Background(), req, mockDynamo, "test-table")
	assert.NoError(t, err)
	assert.Equal(t, 500, resp.StatusCode)
	assert.Equal(t, "dynamo error", resp.Body)
}
