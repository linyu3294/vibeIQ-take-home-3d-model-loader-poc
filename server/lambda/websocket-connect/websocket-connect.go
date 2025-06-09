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

	"vibeIQ-take-home-3d-model-loader-poc/lambda/helpers"
)

func HandleConnect(ctx context.Context, req events.APIGatewayWebsocketProxyRequest, dynamo dynamodbiface.DynamoDBAPI, tableName string) (events.APIGatewayProxyResponse, error) {
	if resp, err := helpers.ValidateWebSocketAPIKey(req); err != nil || resp.StatusCode != 0 {
		return resp, err
	}

	if tableName == "" {
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: "connections_table not set"}, nil
	}
	fmt.Printf("Connect event for connectionId: %s\n", req.RequestContext.ConnectionID)
	_, err := dynamo.PutItem(&dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item: map[string]*dynamodb.AttributeValue{
			"connectionId": {S: aws.String(req.RequestContext.ConnectionID)},
		},
	})
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: err.Error()}, nil
	}
	return events.APIGatewayProxyResponse{StatusCode: 200, Body: "Connected"}, nil
}

func handler(ctx context.Context, req events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	sess := session.Must(session.NewSession())
	dynamo := dynamodb.New(sess)
	tableName := os.Getenv("connections_table")
	return HandleConnect(ctx, req, dynamo, tableName)
}

func main() {
	lambda.Start(handler)
}
