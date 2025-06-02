package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

type SuccessResponse struct {
	ModelURL string `json:"modelUrl"`
}

func handler(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	if resp, err := validateAPIKey(req); err != nil || resp.StatusCode != 0 {
		return resp, err
	}

	modelID, exists := req.PathParameters["id"]
	if !exists {
		errorResp := ErrorResponse{Error: "Model ID is required"}
		body, _ := json.Marshal(errorResp)
		return events.APIGatewayV2HTTPResponse{
			StatusCode: 400,
			Body:       string(body),
		}, nil
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		errorResp := ErrorResponse{Error: fmt.Sprintf("Failed to load AWS config: %v", err)}
		body, _ := json.Marshal(errorResp)
		return events.APIGatewayV2HTTPResponse{
			StatusCode: 500,
			Body:       string(body),
		}, nil
	}

	s3Client := s3.NewFromConfig(cfg)

	objectKey := fmt.Sprintf("%s.glb", modelID)
	presignClient := s3.NewPresignClient(s3Client)
	presignedURL, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(os.Getenv("model_s3_bucket")),
		Key:    aws.String(objectKey),
	}, s3.WithPresignExpires(time.Duration(24)*time.Hour)) // URL expires in 24 hours

	if err != nil {
		errorResp := ErrorResponse{Error: fmt.Sprintf("Failed to generate presigned URL: %v", err)}
		body, _ := json.Marshal(errorResp)
		return events.APIGatewayV2HTTPResponse{
			StatusCode: 500,
			Body:       string(body),
		}, nil
	}

	successResp := SuccessResponse{ModelURL: presignedURL.URL}
	body, _ := json.Marshal(successResp)
	return events.APIGatewayV2HTTPResponse{
		StatusCode: 200,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: string(body),
	}, nil
}

func validateAPIKey(req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	apiKey := req.Headers["x-api-key"]
	if apiKey == "" {
		errorResp := ErrorResponse{Error: "API key is required"}
		body, _ := json.Marshal(errorResp)
		return events.APIGatewayV2HTTPResponse{
			StatusCode: 401,
			Body:       string(body),
		}, nil
	}

	if apiKey != os.Getenv("api_key_value") {
		errorResp := ErrorResponse{Error: "Invalid API key"}
		body, _ := json.Marshal(errorResp)
		return events.APIGatewayV2HTTPResponse{
			StatusCode: 403,
			Body:       string(body),
		}, nil
	}

	return events.APIGatewayV2HTTPResponse{}, nil
}

func main() {
	lambda.Start(handler)
}
