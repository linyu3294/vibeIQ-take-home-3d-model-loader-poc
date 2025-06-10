#!/bin/bash

# Build each Lambda function
echo "Building Lambda functions..."

cd ./lambda


# Build model-loader-util function
echo "Building model-loader-util function..."
cd ./model-loader-util
echo "Creating zip file for model-loader-util.go function..."
GOOS=linux GOARCH=amd64 go build -o bootstrap model-loader-util.go
zip ./model-loader-util.zip bootstrap

cd ../

# Build the websocket-connect function
echo "Building websocket-connect function..."
cd ./websocket-connect
echo "Creating zip file for websocket-connect.go function..."
GOOS=linux GOARCH=amd64 go build -o bootstrap websocket-connect.go
zip ./websocket-connect.zip bootstrap

cd ../

# Build websocket-disconnect function
echo "Building websocket-disconnect function..."
cd ./websocket-disconnect
echo "Creating zip file for websocket-disconnect.go function..."
GOOS=linux GOARCH=amd64 go build -o bootstrap websocket-disconnect.go
zip ./websocket-disconnect.zip bootstrap

cd ../

# Build websocket-default function
echo "Building websocket-default function..."
cd ./websocket-default
echo "Creating zip file for websocket-default.go function..."
GOOS=linux GOARCH=amd64 go build -o bootstrap websocket-default.go
zip ./websocket-default.zip bootstrap

cd ../

# Build notification function
echo "Building notification function..."
cd ./notification
echo "Creating zip file for notification.go function..."
GOOS=linux GOARCH=amd64 go build -o bootstrap notification.go
zip ./notification.zip bootstrap


cd ../../

# Deploy to AWS
echo "Deploying to AWS..."
terraform apply 

echo "Build complete! Lambda functions are ready for deployment."