package helpers

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/aws/aws-lambda-go/events"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

func validateAPIKey(apiKey string) (int, string) {
	fmt.Printf("Validating API key. Received key: %s, Expected key: %s\n", apiKey, os.Getenv("api_key_value"))

	if apiKey == "" {
		errorResp := ErrorResponse{Error: "API key is required"}
		body, _ := json.Marshal(errorResp)
		return 401, string(body)
	}

	if apiKey != os.Getenv("api_key_value") {
		errorResp := ErrorResponse{Error: fmt.Sprintf("Invalid API key. Received: %s", apiKey)}
		body, _ := json.Marshal(errorResp)
		return 403, string(body)
	}

	return 0, ""
}

func ValidateHttpAPIKey(req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	apiKey := req.Headers["x-api-key"]
	statusCode, body := validateAPIKey(apiKey)
	if statusCode != 0 {
		return events.APIGatewayV2HTTPResponse{
			StatusCode: statusCode,
			Body:       body,
		}, nil
	}
	return events.APIGatewayV2HTTPResponse{}, nil
}

func ValidateWebSocketAPIKey(req events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	fmt.Printf("WebSocket request query parameters: %v\n", req.QueryStringParameters)
	apiKey := req.QueryStringParameters["apiKey"]
	statusCode, body := validateAPIKey(apiKey)
	if statusCode != 0 {
		return events.APIGatewayProxyResponse{
			StatusCode: statusCode,
			Body:       body,
		}, nil
	}
	return events.APIGatewayProxyResponse{}, nil
}
