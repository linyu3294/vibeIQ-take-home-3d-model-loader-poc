package main

import (
	"context"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

func handler(ctx context.Context, req events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Log the incoming message and connection ID
	fmt.Printf("Received message: %s\n", req.Body)
	fmt.Printf("Connection ID: %s\n", req.RequestContext.ConnectionID)

	// You can add logic here to handle different message types, actions, etc.

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       "Message received",
	}, nil
}

func main() {
	lambda.Start(handler)
}
