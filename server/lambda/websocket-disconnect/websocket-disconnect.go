package main

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
)

func HandleDisconnect(ctx context.Context, req events.APIGatewayWebsocketProxyRequest, dynamo dynamodbiface.DynamoDBAPI, tableName string) (events.APIGatewayProxyResponse, error) {
	if tableName == "" {
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: "CONNECTIONS_TABLE not set"}, nil
	}
	fmt.Printf("Disconnect event for connectionId: %s\n", req.RequestContext.ConnectionID)
	_, err := dynamo.DeleteItem(&dynamodb.DeleteItemInput{
		TableName: aws.String(tableName),
		Key: map[string]*dynamodb.AttributeValue{
			"connectionId": {S: aws.String(req.RequestContext.ConnectionID)},
		},
	})
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: err.Error()}, nil
	}
	return events.APIGatewayProxyResponse{StatusCode: 200, Body: "Disconnected"}, nil
}

func handler(ctx context.Context, req events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	sess := session.Must(session.NewSession())
	dynamo := dynamodb.New(sess)
	tableName := os.Getenv("CONNECTIONS_TABLE")
	return HandleDisconnect(ctx, req, dynamo, tableName)
}

func main() {
	lambda.Start(handler)
}
