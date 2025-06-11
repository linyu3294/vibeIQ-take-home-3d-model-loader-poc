package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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

type SuccessGetModelResponse struct {
	PresignedUrl string `json:"presignedUrl"`
}

type SuccessPutResponse struct {
	Message string `json:"message"`
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
GET /v1/3d-model/{unique-model-id}?getPresignedUploadURL={boolean}&fileType={string}
###########################################
*/

func HandleGetModelRequest(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
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

	shouldGetPresignedUploadURL := request.QueryStringParameters["getPresignedUploadURL"]
	fileType := request.QueryStringParameters["fileType"]
	if fileType == "" {
		return createErrorResponse(400, "Malformed request - fileType query parameter is required"), nil
	}
	if shouldGetPresignedUploadURL == "true" && fileType != "blend" {
		return createErrorResponse(400, "Malformed request - fileType query parameter is not supported"), nil
	}
	if (shouldGetPresignedUploadURL == "false" || shouldGetPresignedUploadURL == "") && fileType != "glb" {
		return createErrorResponse(400, "Malformed request - fetching this file type is not supported"), nil
	}

	objectKey := fmt.Sprintf("%s/%s.%s", fileType, modelID, fileType)

	if shouldGetPresignedUploadURL == "true" {
		presignedURL, err := presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(objectKey),
		},
			// Set to 1 minute for getting presigned URL to upload to S3
			// The presigned URL is used to upload the file to S3 immediately
			s3.WithPresignExpires(1*time.Minute))
		if err != nil {
			return createErrorResponse(500, "Failed to generate presigned PUT URL"), err
		}
		successResp := SuccessGetModelResponse{PresignedUrl: presignedURL.URL}
		return createSuccessResponse(200, successResp), nil
	}

	presignedURL, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(objectKey),
	}, s3.WithPresignExpires(time.Duration(24)*time.Hour))

	if err != nil {
		return createErrorResponse(500, "Failed to generate presigned URL"), err
	}

	successResp := SuccessGetModelResponse{PresignedUrl: presignedURL.URL}
	return createSuccessResponse(200, successResp), nil
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayV2HTTPResponse, error) {
	log.Println("Received request:", request)
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
		RawPath:               request.Path,
	}
	log.Printf("Converted request path: %s", req.RawPath)
	switch strings.ToUpper(req.RequestContext.HTTP.Method) {
	case "GET":
		if strings.Contains(req.RawPath, "/3d-model/") {
			return HandleGetModelRequest(ctx, req)
		}
		return createErrorResponse(404, "Not found"), nil
	case "POST":
		cfg, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			return createErrorResponse(500, "Error loading AWS config"), err
		}

		sqsClient := sqs.NewFromConfig(cfg)
		return HandlePostRequest(ctx, req, sqsClient)
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
