#!/bin/bash

# Build each Lambda function
echo "Building Lambda functions..."

# Build model-loader-util function
echo "Building model-loader-util function..."

cd ./lambda

# Build model-loader-util function
cd ./model-loader-util
echo "Creating zip file for model-loader-util.go function..."
GOOS=linux GOARCH=amd64 go build -o bootstrap model-loader-util.go
zip ./model-loader-util.zip bootstrap

cd ../../

# Deploy to AWS
echo "Deploying to AWS..."
terraform apply 

echo "Build complete! Lambda functions are ready for deployment."