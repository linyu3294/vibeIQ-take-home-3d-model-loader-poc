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
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"vibeIQ-take-home-3d-model-loader-poc/lambda/helpers"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

type SuccessResponse struct {
	ModelURL string `json:"modelUrl"`
	JobID    string `json:"jobId,omitempty"`
}

type ConversionJob struct {
	JobType      string `json:"jobType"`
	FromFileType string `json:"fromFileType"`
	ToFileType   string `json:"toFileType"`
	ModelID      string `json:"modelId"`
}

func handler(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	if resp, err := helpers.ValidateHttpAPIKey(req); err != nil || resp.StatusCode != 0 {
		return resp, err
	}

	// Load AWS config
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return errorResponse(500, fmt.Sprintf("Failed to load AWS config: %v", err))
	}

	// Handle GET request
	if req.RequestContext.HTTP.Method == "GET" {
		return handleGetRequest(ctx, cfg, req)
	}

	// Handle POST request
	if req.RequestContext.HTTP.Method == "POST" {
		return handlePostRequest(ctx, cfg, req)
	}

	return errorResponse(400, "Unsupported HTTP method")
}

func handleGetRequest(ctx context.Context, cfg aws.Config, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	modelID, exists := req.PathParameters["id"]
	if !exists {
		return errorResponse(400, "Model ID is required")
	}

	s3Client := s3.NewFromConfig(cfg)
	objectKey := fmt.Sprintf("%s.glb", modelID)
	presignClient := s3.NewPresignClient(s3Client)
	presignedURL, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(os.Getenv("model_s3_bucket")),
		Key:    aws.String(objectKey),
	}, s3.WithPresignExpires(time.Duration(24)*time.Hour))

	if err != nil {
		return errorResponse(500, fmt.Sprintf("Failed to generate presigned URL: %v", err))
	}

	successResp := SuccessResponse{ModelURL: presignedURL.URL}
	return jsonResponse(200, successResp)
}

func handlePostRequest(ctx context.Context, cfg aws.Config, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	modelID, exists := req.PathParameters["id"]
	if !exists {
		return errorResponse(400, "Model ID is required")
	}

	// Create conversion job
	job := ConversionJob{
		JobType:      "conversion",
		FromFileType: ".blend",
		ToFileType:   ".glb",
		ModelID:      modelID,
	}

	// Enqueue job to SQS
	sqsClient := sqs.NewFromConfig(cfg)
	queueURL := os.Getenv("sqs_queue_url")
	if queueURL == "" {
		return errorResponse(500, "sqs_queue_url environment variable not set")
	}

	jobBody, err := json.Marshal(job)
	if err != nil {
		return errorResponse(500, fmt.Sprintf("Failed to marshal job: %v", err))
	}

	_, err = sqsClient.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(queueURL),
		MessageBody: aws.String(string(jobBody)),
	})
	if err != nil {
		return errorResponse(500, fmt.Sprintf("Failed to enqueue job: %v", err))
	}

	successResp := SuccessResponse{
		JobID: modelID,
	}
	return jsonResponse(202, successResp)
}

func errorResponse(statusCode int, message string) (events.APIGatewayV2HTTPResponse, error) {
	errorResp := ErrorResponse{Error: message}
	body, _ := json.Marshal(errorResp)
	return events.APIGatewayV2HTTPResponse{
		StatusCode: statusCode,
		Body:       string(body),
	}, nil
}

func jsonResponse(statusCode int, body interface{}) (events.APIGatewayV2HTTPResponse, error) {
	jsonBody, _ := json.Marshal(body)
	return events.APIGatewayV2HTTPResponse{
		StatusCode: statusCode,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: string(jsonBody),
	}, nil
}

func main() {
	lambda.Start(handler)
}
