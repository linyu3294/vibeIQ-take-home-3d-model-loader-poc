package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/stretchr/testify/assert"
)

type mockSQSClient struct {
	sendMessageInput *sqs.SendMessageInput
	sendMessageErr   error
}

func (m *mockSQSClient) SendMessage(ctx context.Context, params *sqs.SendMessageInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error) {
	m.sendMessageInput = params
	return &sqs.SendMessageOutput{}, m.sendMessageErr
}

func TestHandlePostRequest_SuccessQueueJob(t *testing.T) {
	os.Setenv("api_key_value", "test-api-key")
	os.Setenv("blender_jobs_queue_url", "https://sqs.us-east-1.amazonaws.com/123456789012/test-queue")
	defer func() {
		os.Unsetenv("api_key_value")
		os.Unsetenv("blender_jobs_queue_url")
	}()

	mockSQS := &mockSQSClient{}

	req := events.APIGatewayV2HTTPRequest{
		Headers: map[string]string{
			"x-api-key":    "test-api-key",
			"Content-Type": "application/json",
		},
		RequestContext: events.APIGatewayV2HTTPRequestContext{
			HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
				Method: "POST",
			},
		},
		Body: `{
			"connectionId": "test-connection-id",
			"fromFileType": "blend",
			"toFileType": "glb",
			"modelId": "test-model-id",
			"s3Key": "test-s3-key"
		}`,
	}

	resp, err := HandlePostRequest(context.Background(), req, mockSQS)
	assert.NoError(t, err)
	assert.Equal(t, 202, resp.StatusCode)

	var response SuccessPostResponse
	err = json.Unmarshal([]byte(resp.Body), &response)
	assert.NoError(t, err)
	assert.Equal(t, "Job successfully queued", response.Status)

	assert.NotNil(t, mockSQS.sendMessageInput)
	assert.Equal(t, "https://sqs.us-east-1.amazonaws.com/123456789012/test-queue", *mockSQS.sendMessageInput.QueueUrl)

	var messageBody map[string]string
	err = json.Unmarshal([]byte(*mockSQS.sendMessageInput.MessageBody), &messageBody)
	assert.NoError(t, err)
	assert.Equal(t, "conversion", messageBody["jobType"])
	assert.Equal(t, "pending", messageBody["jobStatus"])
	assert.Equal(t, "test-connection-id", messageBody["connectionId"])
	assert.Equal(t, "blend", messageBody["fromFileType"])
	assert.Equal(t, "glb", messageBody["toFileType"])
	assert.Equal(t, "test-model-id", messageBody["modelId"])
	assert.Equal(t, "test-s3-key", messageBody["s3Key"])
}

func TestHandlePostRequest_MissingAPIKey(t *testing.T) {
	os.Setenv("api_key_value", "test-api-key")
	os.Setenv("blender_jobs_queue_url", "https://sqs.us-east-1.amazonaws.com/123456789012/test-queue")
	defer func() {
		os.Unsetenv("api_key_value")
		os.Unsetenv("blender_jobs_queue_url")
	}()

	mockSQS := &mockSQSClient{}

	req := events.APIGatewayV2HTTPRequest{
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		RequestContext: events.APIGatewayV2HTTPRequestContext{
			HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
				Method: "POST",
			},
		},
		Body: `{
			"connectionId": "test-connection-id",
			"fromFileType": "blend",
			"toFileType": "glb",
			"modelId": "test-model-id",
			"s3Key": "test-s3-key"
		}`,
	}

	resp, err := HandlePostRequest(context.Background(), req, mockSQS)
	assert.NoError(t, err)

	log.Printf("resp: %+v", resp)

	assert.Equal(t, 401, resp.StatusCode)
}

func TestHandlePostRequest_InvalidAPIKey(t *testing.T) {
	os.Setenv("api_key_value", "test-api-key")
	os.Setenv("blender_jobs_queue_url", "https://sqs.us-east-1.amazonaws.com/123456789012/test-queue")
	defer func() {
		os.Unsetenv("api_key_value")
		os.Unsetenv("blender_jobs_queue_url")
	}()

	mockSQS := &mockSQSClient{}

	req := events.APIGatewayV2HTTPRequest{
		Headers: map[string]string{
			"x-api-key":    "incorrect-api-key",
			"Content-Type": "application/json",
		},
		RequestContext: events.APIGatewayV2HTTPRequestContext{
			HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
				Method: "POST",
			},
		},
		Body: `{
			"connectionId": "test-connection-id",
			"fromFileType": "blend",
			"toFileType": "glb",
			"modelId": "test-model-id",
			"s3Key": "test-s3-key"
		}`,
	}

	resp, err := HandlePostRequest(context.Background(), req, mockSQS)
	assert.NoError(t, err)

	log.Printf("resp: %+v", resp)

	assert.Equal(t, 403, resp.StatusCode)
}

func TestHandlePostRequest_MissingRequiredBody_Returns400(t *testing.T) {
	os.Setenv("api_key_value", "test-api-key")
	os.Setenv("blender_jobs_queue_url", "https://sqs.us-east-1.amazonaws.com/123456789012/test-queue")
	defer func() {
		os.Unsetenv("api_key_value")
		os.Unsetenv("blender_jobs_queue_url")
	}()

	mockSQS := &mockSQSClient{}

	req1 := events.APIGatewayV2HTTPRequest{
		Headers: map[string]string{
			"x-api-key":    "test-api-key",
			"Content-Type": "application/json",
		},
		RequestContext: events.APIGatewayV2HTTPRequestContext{
			HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
				Method: "POST",
			},
		},
		Body: `{
			"fromFileType": "blend",
			"toFileType": "glb",
			"modelId": "test-model-id",
			"s3Key": "test-s3-key"
		}`,
	}

	resp1, err1 := HandlePostRequest(context.Background(), req1, mockSQS)
	log.Printf("resp1: %+v", resp1)
	assert.NoError(t, err1)
	assert.Equal(t, "{\"error\":\"Missing required fields: connectionId\"}", resp1.Body)

	req2 := events.APIGatewayV2HTTPRequest{
		Headers: map[string]string{
			"x-api-key":    "test-api-key",
			"Content-Type": "application/json",
		},
		RequestContext: events.APIGatewayV2HTTPRequestContext{
			HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
				Method: "POST",
			},
		},
		Body: `{
			"connectionId": "test-connection-id",
			"toFileType": "glb",
			"modelId": "test-model-id",
			"s3Key": "test-s3-key"
		}`,
	}

	resp2, err2 := HandlePostRequest(context.Background(), req2, mockSQS)
	assert.NoError(t, err2)
	assert.Equal(t, 400, resp2.StatusCode)
	assert.Equal(t, "{\"error\":\"Missing required fields: fromFileType\"}", resp2.Body)

	req3 := events.APIGatewayV2HTTPRequest{
		Headers: map[string]string{
			"x-api-key":    "test-api-key",
			"Content-Type": "application/json",
		},
		RequestContext: events.APIGatewayV2HTTPRequestContext{
			HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
				Method: "POST",
			},
		},
		Body: `{
			"connectionId": "test-connection-id",
			"fromFileType": "blend",
			"modelId": "test-model-id",
			"s3Key": "test-s3-key"
		}`,
	}

	resp3, err3 := HandlePostRequest(context.Background(), req3, mockSQS)
	assert.NoError(t, err3)
	assert.Equal(t, 400, resp3.StatusCode)
	assert.Equal(t, "{\"error\":\"Missing required fields: toFileType\"}", resp3.Body)

	req4 := events.APIGatewayV2HTTPRequest{
		Headers: map[string]string{
			"x-api-key":    "test-api-key",
			"Content-Type": "application/json",
		},
		RequestContext: events.APIGatewayV2HTTPRequestContext{
			HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
				Method: "POST",
			},
		},
		Body: `{
			"connectionId": "test-connection-id",
			"fromFileType": "blend",
			"toFileType": "glb",
			"s3Key": "test-s3-key"
		}`,
	}

	resp4, err4 := HandlePostRequest(context.Background(), req4, mockSQS)
	assert.NoError(t, err4)
	assert.Equal(t, 400, resp4.StatusCode)
	assert.Equal(t, "{\"error\":\"Missing required fields: modelId\"}", resp4.Body)

	req5 := events.APIGatewayV2HTTPRequest{
		Headers: map[string]string{
			"x-api-key":    "test-api-key",
			"Content-Type": "application/json",
		},
		RequestContext: events.APIGatewayV2HTTPRequestContext{
			HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
				Method: "POST",
			},
		},
		Body: `{
			"connectionId": "test-connection-id",
			"fromFileType": "blend",
			"toFileType": "glb",
			"modelId": "test-model-id"
		}`,
	}

	resp5, err5 := HandlePostRequest(context.Background(), req5, mockSQS)
	assert.NoError(t, err5)
	assert.Equal(t, 400, resp5.StatusCode)
	log.Printf("resp5: %+v", resp5)
	assert.Equal(t, "{\"error\":\"Missing required fields: s3Key\"}", resp5.Body)
}

func TestHandlePostRequest_InvalidFromFileType_Returns400(t *testing.T) {
	os.Setenv("api_key_value", "test-api-key")
	os.Setenv("blender_jobs_queue_url", "https://sqs.us-east-1.amazonaws.com/123456789012/test-queue")
	defer func() {
		os.Unsetenv("api_key_value")
		os.Unsetenv("blender_jobs_queue_url")
	}()

	mockSQS := &mockSQSClient{}

	req := events.APIGatewayV2HTTPRequest{
		Headers: map[string]string{
			"x-api-key":    "test-api-key",
			"Content-Type": "application/json",
		},
		RequestContext: events.APIGatewayV2HTTPRequestContext{
			HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
				Method: "POST",
			},
		},
		Body: `{
			"connectionId": "test-connection-id",
			"fromFileType": "word-document",
			"toFileType": "glb",
			"modelId": "test-model-id",
			"s3Key": "test-s3-key"
		}`,
	}

	resp, err := HandlePostRequest(context.Background(), req, mockSQS)
	assert.NoError(t, err)

	log.Printf("resp: %+v", resp)

	assert.Equal(t, 400, resp.StatusCode)
	assert.Equal(t, "{\"error\":\"Only blend files are supported\"}", resp.Body)
}

func TestHandlePostRequest_InvalidToFileType_Returns400(t *testing.T) {
	os.Setenv("api_key_value", "test-api-key")
	os.Setenv("blender_jobs_queue_url", "https://sqs.us-east-1.amazonaws.com/123456789012/test-queue")
	defer func() {
		os.Unsetenv("api_key_value")
		os.Unsetenv("blender_jobs_queue_url")
	}()

	mockSQS := &mockSQSClient{}

	req := events.APIGatewayV2HTTPRequest{
		Headers: map[string]string{
			"x-api-key":    "test-api-key",
			"Content-Type": "application/json",
		},
		RequestContext: events.APIGatewayV2HTTPRequestContext{
			HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
				Method: "POST",
			},
		},
		Body: `{
			"connectionId": "test-connection-id",
			"fromFileType": "blend",
			"toFileType": "docx",
			"modelId": "test-model-id",
			"s3Key": "test-s3-key"
		}`,
	}

	resp, err := HandlePostRequest(context.Background(), req, mockSQS)
	assert.NoError(t, err)

	log.Printf("resp: %+v", resp)

	assert.Equal(t, 400, resp.StatusCode)
	assert.Equal(t, "{\"error\":\"Only glb, gltf, obj, fbx, usd, usdz files are supported\"}", resp.Body)
}

func TestHandlePostRequest_SQSError_Returns500(t *testing.T) {
	os.Setenv("api_key_value", "test-api-key")
	os.Setenv("blender_jobs_queue_url", "https://sqs.us-east-1.amazonaws.com/123456789012/test-queue")
	defer func() {
		os.Unsetenv("api_key_value")
		os.Unsetenv("blender_jobs_queue_url")
	}()

	// Create a mock SQS client that returns an error
	mockSQS := &mockSQSClient{
		sendMessageErr: errors.New("failed to send message to SQS"),
	}

	req := events.APIGatewayV2HTTPRequest{
		Headers: map[string]string{
			"x-api-key":    "test-api-key",
			"Content-Type": "application/json",
		},
		RequestContext: events.APIGatewayV2HTTPRequestContext{
			HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
				Method: "POST",
			},
		},
		Body: `{
			"connectionId": "test-connection-id",
			"fromFileType": "blend",
			"toFileType": "glb",
			"modelId": "test-model-id",
			"s3Key": "test-s3-key"
		}`,
	}

	resp, err := HandlePostRequest(context.Background(), req, mockSQS)

	log.Printf("resp: %+v", resp)
	assert.Error(t, err)
	assert.Equal(t, 500, resp.StatusCode)
	assert.Equal(t, "{\"error\":\"Error sending message to queue\"}", resp.Body)
}

func TestHandleGetModelRequest_Success(t *testing.T) {
	os.Setenv("api_key_value", "test-api-key")
	os.Setenv("model_s3_bucket", "test-bucket")
	defer func() {
		os.Unsetenv("api_key_value")
		os.Unsetenv("model_s3_bucket")
	}()

	req1 := events.APIGatewayV2HTTPRequest{
		Headers: map[string]string{
			"x-api-key":    "test-api-key",
			"Content-Type": "application/json",
		},
		PathParameters: map[string]string{
			"id": "test-model-id",
		},
		QueryStringParameters: map[string]string{
			"fileType":              "blend",
			"getPresignedUploadURL": "true",
		},
	}

	resp1, err1 := HandleGetModelRequest(context.Background(), req1)
	assert.NoError(t, err1)
	log.Printf("resp1: %+v", resp1)
	assert.Equal(t, 200, resp1.StatusCode)

	// Simplified regex to match the essential parts of the presigned URL
	expectedPattern1 := `^{"presignedUrl":"https://test-bucket\.s3\.us-east-1\.amazonaws\.com/blend/test-model-id\.blend\?.*"}$`
	assert.Regexp(t, expectedPattern1, resp1.Body)

	req2 := events.APIGatewayV2HTTPRequest{
		Headers: map[string]string{
			"x-api-key":    "test-api-key",
			"Content-Type": "application/json",
		},
		PathParameters: map[string]string{
			"id": "test-model-id",
		},
		QueryStringParameters: map[string]string{
			"fileType":              "glb",
			"getPresignedUploadURL": "false",
		},
	}

	resp2, err2 := HandleGetModelRequest(context.Background(), req2)
	assert.NoError(t, err2)
	log.Printf("resp1: %+v", resp2)
	assert.Equal(t, 200, resp1.StatusCode)

	// Simplified regex to match the essential parts of the presigned URL
	expectedPattern2 := `^{"presignedUrl":"https://test-bucket\.s3\.us-east-1\.amazonaws\.com/glb/test-model-id\.glb\?.*"}$`
	assert.Regexp(t, expectedPattern2, resp2.Body)
}
