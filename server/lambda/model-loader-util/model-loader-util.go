package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
	"vibeIQ-take-home-3d-model-loader-poc/lambda/helpers"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
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

type SuccessGetModelsResponse struct {
	Models     []ModelMetadata `json:"models"`
	NextCursor string          `json:"nextCursor,omitempty"`
}

type ModelMetadata struct {
	JobID        string `json:"jobId"`
	ConnectionID string `json:"connectionId"`
	JobType      string `json:"jobType"`
	JobStatus    string `json:"jobStatus"`
	FromFileType string `json:"fromFileType"`
	ToFileType   string `json:"toFileType"`
	ModelID      string `json:"modelId"`
	S3Key        string `json:"s3Key"`
	NewS3Key     string `json:"newS3Key,omitempty"`
	Error        string `json:"error,omitempty"`
	Timestamp    string `json:"timestamp"`
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

type DynamoDBClient interface {
	Query(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
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

/*
###########################################
GET /v1/3d-models?fileType{string}&limit={number}&cursor={string}
###########################################
*/

func HandleGetModelsRequest(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	apiKeyResp, err := helpers.ValidateHttpAPIKey(request)
	if err != nil {
		return createErrorResponse(500, "Error validating API key"), err
	}
	if apiKeyResp.StatusCode != 0 {
		return apiKeyResp, nil
	}

	fileType := request.QueryStringParameters["fileType"]
	limitStr := request.QueryStringParameters["limit"]
	cursor := request.QueryStringParameters["cursor"]

	limit := 10
	if limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit <= 0 || limit > 100 {
			return createErrorResponse(400, "Invalid limit parameter. Must be a positive number between 1 and 100"), nil
		}
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return createErrorResponse(500, "Failed to load AWS config"), err
	}

	dynamoClient := dynamodb.NewFromConfig(cfg)
	tableName := os.Getenv("job_history_table")

	models := make([]ModelMetadata, 0, limit)
	var lastEvaluatedKey map[string]types.AttributeValue
	if cursor != "" {
		// First decode the URL-encoded cursor
		decodedCursor, err := url.QueryUnescape(cursor)
		if err != nil {
			return createErrorResponse(400, "Invalid URL-encoded cursor"), nil
		}

		// Parse into a temporary map
		var tempMap map[string]interface{}
		if err := json.Unmarshal([]byte(decodedCursor), &tempMap); err != nil {
			return createErrorResponse(400, "Invalid cursor format"), nil
		}

		// Convert to DynamoDB AttributeValue format
		lastEvaluatedKey = make(map[string]types.AttributeValue)
		for k, v := range tempMap {
			if strVal, ok := v.(string); ok {
				lastEvaluatedKey[k] = &types.AttributeValueMemberS{Value: strVal}
			}
		}
	}

	for len(models) < limit {
		queryInput := &dynamodb.QueryInput{
			TableName: aws.String(tableName),
			Limit:     aws.Int32(int32(limit - len(models) + 1)), // fetch a bit more to check for more pages
		}

		if fileType != "" {
			queryInput.IndexName = aws.String("ToFileTypeIndex")
			queryInput.KeyConditionExpression = aws.String("toFileType = :fileType")
			queryInput.ExpressionAttributeValues = map[string]types.AttributeValue{
				":fileType": &types.AttributeValueMemberS{Value: fileType},
			}
		}
		if lastEvaluatedKey != nil {
			queryInput.ExclusiveStartKey = lastEvaluatedKey
		}

		result, err := dynamoClient.Query(ctx, queryInput)
		if err != nil {
			return createErrorResponse(500, "Failed to query models"), err
		}

		for _, item := range result.Items {
			jobStatus := item["jobStatus"].(*types.AttributeValueMemberS).Value
			if jobStatus == "failed" {
				continue
			}
			model := ModelMetadata{
				JobID:        item["jobId"].(*types.AttributeValueMemberS).Value,
				ConnectionID: item["connectionId"].(*types.AttributeValueMemberS).Value,
				JobType:      item["jobType"].(*types.AttributeValueMemberS).Value,
				JobStatus:    jobStatus,
				FromFileType: item["fromFileType"].(*types.AttributeValueMemberS).Value,
				ToFileType:   item["toFileType"].(*types.AttributeValueMemberS).Value,
				ModelID:      item["modelId"].(*types.AttributeValueMemberS).Value,
				S3Key:        item["s3Key"].(*types.AttributeValueMemberS).Value,
				Timestamp:    item["timestamp"].(*types.AttributeValueMemberS).Value,
			}
			if newS3Key, ok := item["newS3Key"]; ok {
				model.NewS3Key = newS3Key.(*types.AttributeValueMemberS).Value
			}
			if errorMsg, ok := item["error"]; ok {
				model.Error = errorMsg.(*types.AttributeValueMemberS).Value
			}
			models = append(models, model)
			if len(models) == limit {
				break
			}
		}

		if len(models) == limit || result.LastEvaluatedKey == nil {
			lastEvaluatedKey = result.LastEvaluatedKey
			break
		}
		lastEvaluatedKey = result.LastEvaluatedKey
	}

	response := SuccessGetModelsResponse{
		Models: models,
	}
	if lastEvaluatedKey != nil && len(models) == limit {
		// Convert DynamoDB AttributeValue to simple map for cursor
		cursorMap := make(map[string]string)
		for k, v := range lastEvaluatedKey {
			if s, ok := v.(*types.AttributeValueMemberS); ok {
				cursorMap[k] = s.Value
			}
		}
		nextCursor, err := json.Marshal(cursorMap)
		if err != nil {
			return createErrorResponse(500, "Failed to generate next cursor"), err
		}
		response.NextCursor = string(nextCursor)
	}

	return createSuccessResponse(200, response), nil
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
		if strings.Contains(req.RawPath, "/3d-models") {
			return HandleGetModelsRequest(ctx, req)
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
