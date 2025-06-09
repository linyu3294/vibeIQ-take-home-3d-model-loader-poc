package main

import (
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

type SuccessResponse struct {
	ModelURL string `json:"modelUrl"`
}

type ConversionRequest struct {
	FromFileType string `json:"fromFileType"`
	ToFileType   string `json:"toFileType"`
	ModelID      string `json:"modelId"`
	S3Key        string `json:"s3Key"`
}

func validateAPIKey(apiKey string) bool {
	return apiKey == os.Getenv("API_KEY_VALUE")
}

func handlePostRequest(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	contentType := request.Headers["Content-Type"]
	if contentType == "" {
		contentType = request.Headers["content-type"]
	}

	if strings.HasPrefix(contentType, "multipart/form-data") {
		// Handle file upload
		boundary := ""
		for _, v := range strings.Split(contentType, ";") {
			v = strings.TrimSpace(v)
			if strings.HasPrefix(v, "boundary=") {
				boundary = strings.TrimPrefix(v, "boundary=")
			}
		}
		if boundary == "" {
			return events.APIGatewayProxyResponse{StatusCode: 400, Body: "Missing boundary in multipart/form-data"}, nil
		}
		reader := multipart.NewReader(strings.NewReader(request.Body), boundary)
		form, err := reader.ReadForm(10 << 20) // 10 MB max memory
		if err != nil {
			return events.APIGatewayProxyResponse{StatusCode: 400, Body: "Invalid form data"}, nil
		}
		defer form.RemoveAll()

		fileHeaders := form.File["file"]
		if len(fileHeaders) == 0 {
			return events.APIGatewayProxyResponse{StatusCode: 400, Body: "No file uploaded"}, nil
		}
		fileHeader := fileHeaders[0]
		file, err := fileHeader.Open()
		if err != nil {
			return events.APIGatewayProxyResponse{StatusCode: 500, Body: "Failed to open uploaded file"}, nil
		}
		defer file.Close()

		fileType := form.Value["fileType"]
		modelId := form.Value["modelId"]
		toFileType := form.Value["toFileType"]
		if len(fileType) == 0 || len(modelId) == 0 || len(toFileType) == 0 {
			return events.APIGatewayProxyResponse{StatusCode: 400, Body: "Missing fileType, modelId, or toFileType"}, nil
		}

		ext := filepath.Ext(fileHeader.Filename)
		if ext == "" {
			ext = ".blend"
		}
		s3Key := fmt.Sprintf("%s/%s%s", fileType[0], modelId[0], ext)

		cfg, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			return events.APIGatewayProxyResponse{StatusCode: 500, Body: "AWS config error"}, nil
		}
		s3Client := s3.NewFromConfig(cfg)
		bucket := os.Getenv("model_s3_bucket")
		_, err = s3Client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(s3Key),
			Body:   file,
		})
		if err != nil {
			return events.APIGatewayProxyResponse{StatusCode: 500, Body: "Failed to upload to S3"}, nil
		}

		// Send job to SQS
		sqsClient := sqs.NewFromConfig(cfg)
		queueUrl := os.Getenv("blender_jobs_queue_url")
		message := map[string]string{
			"jobType":      "conversion",
			"fromFileType": fileType[0],
			"toFileType":   toFileType[0],
			"modelId":      modelId[0],
			"s3Key":        s3Key,
		}
		messageBody, _ := json.Marshal(message)
		_, err = sqsClient.SendMessage(ctx, &sqs.SendMessageInput{
			QueueUrl:    aws.String(queueUrl),
			MessageBody: aws.String(string(messageBody)),
		})
		if err != nil {
			return events.APIGatewayProxyResponse{StatusCode: 500, Body: "Failed to send SQS job"}, nil
		}

		return events.APIGatewayProxyResponse{
			StatusCode: 200,
			Body:       fmt.Sprintf(`{"s3Key": "%s"}`, s3Key),
		}, nil
	}

	if !validateAPIKey(request.Headers["X-API-Key"]) {
		return events.APIGatewayProxyResponse{
			StatusCode: 401,
			Body:       "Invalid API key",
		}, nil
	}

	var req ConversionRequest
	if err := json.Unmarshal([]byte(request.Body), &req); err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 400,
			Body:       "Invalid request body",
		}, nil
	}

	if req.FromFileType == "" || req.ToFileType == "" || req.ModelID == "" || req.S3Key == "" {
		return events.APIGatewayProxyResponse{
			StatusCode: 400,
			Body:       "Missing required fields",
		}, nil
	}

	message := map[string]string{
		"jobType":      "conversion",
		"fromFileType": req.FromFileType,
		"toFileType":   req.ToFileType,
		"modelId":      req.ModelID,
		"s3Key":        req.S3Key,
	}

	messageBody, err := json.Marshal(message)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       "Error creating message",
		}, err
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       "Error loading AWS config",
		}, err
	}

	sqsClient := sqs.NewFromConfig(cfg)

	_, err = sqsClient.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(os.Getenv("blender_jobs_queue_url")),
		MessageBody: aws.String(string(messageBody)),
	})

	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       "Error sending message to queue",
		}, err
	}

	return events.APIGatewayProxyResponse{
		StatusCode: 202,
		Body:       "Conversion job queued successfully",
	}, nil
}

func handleGetRequest(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Validate API key
	if !validateAPIKey(request.Headers["X-API-Key"]) {
		return events.APIGatewayProxyResponse{
			StatusCode: 401,
			Body:       "Invalid API key",
		}, nil
	}

	modelID, exists := request.PathParameters["id"]
	if !exists {
		errorResp := ErrorResponse{Error: "Model ID is required"}
		body, _ := json.Marshal(errorResp)
		return events.APIGatewayProxyResponse{
			StatusCode: 400,
			Body:       string(body),
		}, nil
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		errorResp := ErrorResponse{Error: fmt.Sprintf("Failed to load AWS config: %v", err)}
		body, _ := json.Marshal(errorResp)
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       string(body),
		}, nil
	}

	s3Client := s3.NewFromConfig(cfg)
	presignClient := s3.NewPresignClient(s3Client)
	bucket := os.Getenv("model_s3_bucket")

	upload := request.QueryStringParameters["upload"]
	fromFileType := request.QueryStringParameters["fromFileType"]

	if upload == "true" && fromFileType != "" {
		// Generate presigned PUT URL for upload
		objectKey := fmt.Sprintf("%s/%s.%s", fromFileType, modelID, fromFileType)
		presignedURL, err := presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(objectKey),
		}, s3.WithPresignExpires(15*time.Minute))
		if err != nil {
			errorResp := ErrorResponse{Error: fmt.Sprintf("Failed to generate presigned PUT URL: %v", err)}
			body, _ := json.Marshal(errorResp)
			return events.APIGatewayProxyResponse{
				StatusCode: 500,
				Body:       string(body),
			}, nil
		}
		resp := map[string]string{"uploadUrl": presignedURL.URL, "s3Key": objectKey}
		body, _ := json.Marshal(resp)
		return events.APIGatewayProxyResponse{
			StatusCode: 200,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       string(body),
		}, nil
	}

	// Default: generate presigned GET URL for download
	objectKey := fmt.Sprintf("converted/%s.glb", modelID)
	presignedURL, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(objectKey),
	}, s3.WithPresignExpires(time.Duration(24)*time.Hour))

	if err != nil {
		errorResp := ErrorResponse{Error: fmt.Sprintf("Failed to generate presigned URL: %v", err)}
		body, _ := json.Marshal(errorResp)
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       string(body),
		}, nil
	}

	successResp := SuccessResponse{ModelURL: presignedURL.URL}
	body, _ := json.Marshal(successResp)
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: string(body),
	}, nil
}

func handlePutRequest(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	fmt.Println("PUT handler invoked")
	fmt.Printf("Headers: %+v\n", request.Headers)
	fmt.Printf("PathParameters: %+v\n", request.PathParameters)
	fmt.Printf("QueryStringParameters: %+v\n", request.QueryStringParameters)
	fmt.Printf("Body size: %d bytes\n", len(request.Body))

	if !validateAPIKey(request.Headers["X-API-Key"]) {
		fmt.Println("Invalid API key")
		return events.APIGatewayProxyResponse{
			StatusCode: 401,
			Body:       "Invalid API key",
		}, nil
	}

	modelID, exists := request.PathParameters["id"]
	if !exists {
		fmt.Println("Model ID is required")
		return events.APIGatewayProxyResponse{
			StatusCode: 400,
			Body:       "Model ID is required",
		}, nil
	}

	fromFileType := request.QueryStringParameters["fromFileType"]
	if fromFileType == "" {
		fmt.Println("fromFileType query parameter is required")
		return events.APIGatewayProxyResponse{
			StatusCode: 400,
			Body:       "fromFileType query parameter is required",
		}, nil
	}

	s3Key := fmt.Sprintf("%s/%s.%s", fromFileType, modelID, fromFileType)
	fmt.Printf("Uploading to S3 bucket: %s, key: %s\n", os.Getenv("model_s3_bucket"), s3Key)

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		fmt.Printf("AWS config error: %v\n", err)
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       "AWS config error",
		}, nil
	}
	s3Client := s3.NewFromConfig(cfg)
	bucket := os.Getenv("model_s3_bucket")
	_, err = s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(s3Key),
		Body:   strings.NewReader(request.Body),
	})
	if err != nil {
		fmt.Printf("Failed to upload to S3: %v\n", err)
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       fmt.Sprintf("Failed to upload to S3: %v", err),
		}, nil
	}

	fmt.Println("Upload to S3 successful")
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       fmt.Sprintf(`{"s3Key": "%s"}`, s3Key),
	}, nil
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	switch request.HTTPMethod {
	case "GET":
		return handleGetRequest(ctx, request)
	case "POST":
		return handlePostRequest(ctx, request)
	case "PUT":
		return handlePutRequest(ctx, request)
	default:
		return events.APIGatewayProxyResponse{
			StatusCode: 405,
			Body:       "Method not allowed",
		}, nil
	}
}

func main() {
	lambda.Start(handler)
}
