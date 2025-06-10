package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"
	"vibeIQ-take-home-3d-model-loader-poc/lambda/helpers"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/google/uuid"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

type SuccessPostResponse struct {
	Status string `json:"status"`
}

type SuccessGetResponse struct {
	PresignedUrl string `json:"presignedUrl"`
}

type ConversionJob struct {
	ConnectionID string `json:"connectionId"`
	FromFileType string `json:"fromFileType"`
	ToFileType   string `json:"toFileType"`
	ModelID      string `json:"modelId"`
	S3Key        string `json:"s3Key"`
}

const (
	contentTypeHeader = "Content-Type"
	apiKeyHeader      = "x-api-key"
	jsonContentType   = "application/json"
)

var supportedOutputFormats = []string{"glb", "gltf", "obj", "fbx", "usd", "usdz"}

type SQSClient interface {
	SendMessage(ctx context.Context, params *sqs.SendMessageInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
}

/*
###########################################
Helper functions
###########################################
*/

func createErrorResponse(statusCode int, message string) events.APIGatewayV2HTTPResponse {
	errorResp := ErrorResponse{Error: message}
	body, _ := json.Marshal(errorResp)
	return events.APIGatewayV2HTTPResponse{
		StatusCode: statusCode,
		Headers:    map[string]string{contentTypeHeader: jsonContentType},
		Body:       string(body),
	}
}

func createSuccessResponse(statusCode int, data interface{}) events.APIGatewayV2HTTPResponse {
	body, _ := json.Marshal(data)
	return events.APIGatewayV2HTTPResponse{
		StatusCode: statusCode,
		Headers:    map[string]string{contentTypeHeader: jsonContentType},
		Body:       string(body),
	}
}

func validateContentType(contentType string) (bool, events.APIGatewayV2HTTPResponse) {
	if strings.HasPrefix(contentType, "multipart/form-data") {
		return false, createErrorResponse(400, "No file uploads allowed")
	}
	return true, events.APIGatewayV2HTTPResponse{}
}

/*
###########################################
POST /v1/3d-model
###########################################
*/

func validateRequiredFieldsForConversion(job ConversionJob) (bool, events.APIGatewayV2HTTPResponse) {
	var missingFields []string
	fields := map[string]string{
		"connectionId": job.ConnectionID,
		"fromFileType": job.FromFileType,
		"toFileType":   job.ToFileType,
		"modelId":      job.ModelID,
		"s3Key":        job.S3Key,
	}

	for name, value := range fields {
		if value == "" {
			missingFields = append(missingFields, name)
		}
	}

	if len(missingFields) > 0 {
		message := fmt.Sprintf("Missing required fields: %s", strings.Join(missingFields, ", "))
		return false, createErrorResponse(400, message)
	}
	return true, events.APIGatewayV2HTTPResponse{}
}

func validateFileTypesForConversion(job ConversionJob) (bool, events.APIGatewayV2HTTPResponse) {
	if job.FromFileType != "blend" {
		return false, createErrorResponse(400, "Only blend files are supported")
	}

	if !slices.Contains(supportedOutputFormats, job.ToFileType) {
		message := fmt.Sprintf("Only %s files are supported", strings.Join(supportedOutputFormats, ", "))
		return false, createErrorResponse(400, message)
	}
	return true, events.APIGatewayV2HTTPResponse{}
}

func handlePostValidations(request events.APIGatewayV2HTTPRequest, job ConversionJob) (events.APIGatewayV2HTTPResponse, error) {
	if valid, resp := validateContentType(request.Headers[contentTypeHeader]); !valid {
		return resp, nil
	}

	apiKeyResp, err := helpers.ValidateHttpAPIKey(request)
	if err != nil {
		return apiKeyResp, err
	}
	if apiKeyResp.StatusCode != 0 {
		return apiKeyResp, nil
	}

	if valid, resp := validateRequiredFieldsForConversion(job); !valid {
		return resp, nil
	}

	if valid, resp := validateFileTypesForConversion(job); !valid {
		return resp, nil
	}

	return events.APIGatewayV2HTTPResponse{
		Headers: map[string]string{contentTypeHeader: jsonContentType},
	}, nil
}

func createConversionMessage(job ConversionJob) map[string]string {
	return map[string]string{
		"jobType":      "conversion",
		"jobId":        uuid.New().String(),
		"jobStatus":    "pending",
		"connectionId": job.ConnectionID,
		"fromFileType": job.FromFileType,
		"toFileType":   job.ToFileType,
		"modelId":      job.ModelID,
		"s3Key":        job.S3Key,
	}
}

func HandlePostRequest(ctx context.Context, request events.APIGatewayV2HTTPRequest, sqsClient SQSClient) (events.APIGatewayV2HTTPResponse, error) {
	var job ConversionJob
	if err := json.Unmarshal([]byte(request.Body), &job); err != nil {
		return createErrorResponse(400, "Invalid request body"), nil
	}
	apiKeyResp, err := helpers.ValidateHttpAPIKey(request)
	if err != nil {
		return createErrorResponse(500, "Error validating API key"), err
	}
	if apiKeyResp.StatusCode != 0 {
		return apiKeyResp, nil
	}

	validations, _ := handlePostValidations(request, job)
	if validations.StatusCode != 0 && validations.StatusCode != 200 {
		return validations, nil
	}

	queueURL := os.Getenv("blender_jobs_queue_url")
	if queueURL == "" {
		return createErrorResponse(500, "Queue URL not configured"), nil
	}

	message := createConversionMessage(job)
	messageBody, err := json.Marshal(message)
	if err != nil {
		return createErrorResponse(500, "Error creating a job queue message"), err
	}

	_, err = sqsClient.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(queueURL),
		MessageBody: aws.String(string(messageBody)),
	})
	if err != nil {
		errorResp := ErrorResponse{Error: "Error sending message to queue"}
		return createSuccessResponse(500, errorResp), err
	}

	successResp := SuccessPostResponse{Status: "Job successfully queued"}
	return createSuccessResponse(202, successResp), nil
}

/*
###########################################
GET /v1/3d-model/{unique-model-id}?upload={boolean}&fileType={string}
###########################################
*/

func handleGetRequest(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	apiKeyResp, err := helpers.ValidateHttpAPIKey(request)
	if err != nil {
		return createErrorResponse(500, "Error validating API key"), err
	}
	if apiKeyResp.StatusCode != 0 {
		return apiKeyResp, nil
	}

	modelID, exists := request.PathParameters["id"]
	if !exists {
		return createErrorResponse(400, "Model id is required"), err
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return createErrorResponse(500, "Failed to load AWS config"), err
	}

	s3Client := s3.NewFromConfig(cfg)
	presignClient := s3.NewPresignClient(s3Client)
	bucket := os.Getenv("model_s3_bucket")

	upload := request.QueryStringParameters["upload"]
	fromFileType := request.QueryStringParameters["fromFileType"]

	if upload == "true" && fromFileType != "" {
		objectKey := fmt.Sprintf("%s/%s.%s", fromFileType, modelID, fromFileType)
		presignedURL, err := presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(objectKey),
		}, s3.WithPresignExpires(15*time.Minute))
		if err != nil {
			return createErrorResponse(500, "Failed to generate presigned PUT URL"), err
		}
		resp := map[string]string{"uploadUrl": presignedURL.URL, "s3Key": objectKey}
		body, _ := json.Marshal(resp)
		return events.APIGatewayV2HTTPResponse{
			StatusCode: 200,
			Headers:    map[string]string{contentTypeHeader: jsonContentType},
			Body:       string(body),
		}, nil
	}

	objectKey := fmt.Sprintf("converted/%s.glb", modelID)
	presignedURL, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(objectKey),
	}, s3.WithPresignExpires(time.Duration(24)*time.Hour))

	if err != nil {
		return createErrorResponse(500, "Failed to generate presigned URL"), err
	}

	successResp := SuccessGetResponse{PresignedUrl: presignedURL.URL}
	body, _ := json.Marshal(successResp)
	return createSuccessResponse(200, body), nil
}

func handlePutRequest(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	apiKeyResp, err := helpers.ValidateHttpAPIKey(request)
	if err != nil {
		return createErrorResponse(500, "Error validating API key"), err
	}
	if apiKeyResp.StatusCode != 0 {
		return apiKeyResp, nil
	}

	modelID, exists := request.PathParameters["id"]
	if !exists {
		return createErrorResponse(400, "Model ID is required"), nil
	}

	fromFileType := request.QueryStringParameters["fromFileType"]
	if fromFileType == "" {
		return createErrorResponse(400, "fromFileType query parameter is required"), nil
	}

	s3Key := fmt.Sprintf("%s/%s.%s", fromFileType, modelID, fromFileType)

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return createErrorResponse(500, "AWS config error"), err
	}
	s3Client := s3.NewFromConfig(cfg)
	bucket := os.Getenv("model_s3_bucket")
	_, err = s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(s3Key),
		Body:   strings.NewReader(request.Body),
	})
	if err != nil {
		return createErrorResponse(500, fmt.Sprintf("Failed to upload to S3: %v", err)), err
	}

	return createSuccessResponse(200, "File uploaded successfully - "), nil
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayV2HTTPResponse, error) {
	req := events.APIGatewayV2HTTPRequest{
		Headers: request.Headers,
		Body:    request.Body,
		RequestContext: events.APIGatewayV2HTTPRequestContext{
			HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
				Method: request.HTTPMethod,
			},
		},
		PathParameters:        request.PathParameters,
		QueryStringParameters: request.QueryStringParameters,
	}
	switch strings.ToUpper(req.RequestContext.HTTP.Method) {
	case "GET":
		return handleGetRequest(ctx, req)
	case "POST":
		cfg, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			return createErrorResponse(500, "Error loading AWS config"), err
		}

		sqsClient := sqs.NewFromConfig(cfg)
		return HandlePostRequest(ctx, req, sqsClient)
	case "PUT":
		return handlePutRequest(ctx, req)
	default:
		return events.APIGatewayV2HTTPResponse{
			StatusCode: 405,
			Headers:    map[string]string{contentTypeHeader: jsonContentType},
			Body:       "Method not allowed",
		}, nil
	}
}

func main() {
	lambda.Start(handler)
}
