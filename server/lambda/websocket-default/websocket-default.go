package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/apigatewaymanagementapi"
)

func sendConnectionIdToClient(ctx context.Context, endpoint, connectionId string) error {
	sess := session.Must(session.NewSession())
	api := apigatewaymanagementapi.New(sess, aws.NewConfig().WithEndpoint(endpoint))
	msg, _ := json.Marshal(map[string]string{"connectionId": connectionId})
	_, err := api.PostToConnectionWithContext(ctx, &apigatewaymanagementapi.PostToConnectionInput{
		ConnectionId: aws.String(connectionId),
		Data:         msg,
	})
	return err
}

func Handler(ctx context.Context, req events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	fmt.Printf("Received message: %s\n", req.Body)
	fmt.Printf("Connection ID: %s\n", req.RequestContext.ConnectionID)

	endpoint := req.RequestContext.DomainName
	if !strings.HasPrefix(endpoint, "https://") && !strings.HasPrefix(endpoint, "http://") {
		endpoint = "https://" + endpoint
	}
	endpoint = endpoint + "/" + req.RequestContext.Stage

	err := sendConnectionIdToClient(ctx, endpoint, req.RequestContext.ConnectionID)
	if err != nil {
		fmt.Printf("Failed to send connectionId to client: %v\n", err)
	}

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       "Message received",
	}, nil
}

func main() {
	lambda.Start(Handler)
}
