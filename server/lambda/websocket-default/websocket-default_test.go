package main

import (
	"context"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/stretchr/testify/assert"
)

func TestHandler_SendsConnectionId(t *testing.T) {
	req := events.APIGatewayWebsocketProxyRequest{
		RequestContext: events.APIGatewayWebsocketProxyRequestContext{
			ConnectionID: "test-connection-id",
			DomainName:   "example.com",
			Stage:        "dev",
		},
		Body: `{"action": "init"}`,
	}

	resp, err := Handler(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "Message received", resp.Body)
}
