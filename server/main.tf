provider "aws" {
  region = var.region
}

locals {
  tags = {
    Project     = var.project_name
    Environment = var.environment
  }
}

data "aws_caller_identity" "current" {}

###########################################
# Shared AWS Resources
###########################################

# IAM role and policies
resource "aws_iam_role" "lambda_exec" {
  name = "${var.project_name}-${var.environment}-lambda-exec-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17",
    Statement = [{
      Action    = "sts:AssumeRole",
      Effect    = "Allow",
      Principal = {
        Service = "lambda.amazonaws.com"
      }
    }]
  })

  tags = local.tags
}

resource "aws_iam_role_policy_attachment" "lambda_policy" {
  role       = aws_iam_role.lambda_exec.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

# IAM policy for S3 access
resource "aws_iam_role_policy" "lambda_s3_policy" {
  name = "${var.project_name}-${var.environment}-lambda-s3-policy"
  role = aws_iam_role.lambda_exec.id

  policy = jsonencode({
    Version = "2012-10-17",
    Statement = [
      {
        Effect = "Allow",
        Action = [
          "s3:ListBucket"
        ],
        Resource = "arn:aws:s3:::${var.model_s3_bucket}"
      },
      {
        Effect = "Allow",
        Action = [
          "s3:GetObject",
          "s3:PutObject",
          "s3:DeleteObject"
        ],
        Resource = "arn:aws:s3:::${var.model_s3_bucket}/*"
      }
    ]
  })
}

# --- HTTP API (v2) Gateway ---
resource "aws_apigatewayv2_api" "model_loader_api" {
  name          = "${var.project_name}-${var.environment}-api"
  protocol_type = "HTTP"
  cors_configuration {
    allow_origins     = concat([var.client_domain], var.allowed_origins)
    allow_methods     = ["GET", "POST", "OPTIONS"]
    allow_headers     = ["Content-Type", "x-api-key", "Authorization"]
    allow_credentials = true
    max_age           = 300
  }
}

resource "aws_apigatewayv2_stage" "model_loader_api_stage" {
  api_id      = aws_apigatewayv2_api.model_loader_api.id
  name        = var.api_stage_name
  auto_deploy = true
}

# --- Lambda Integrations and Routes ---
resource "aws_apigatewayv2_route" "get_model" {
  api_id    = aws_apigatewayv2_api.model_loader_api.id
  route_key = "GET /3d-model/{id}"
  target    = "integrations/${aws_apigatewayv2_integration.get_model.id}"
  authorization_type = "NONE"
}

resource "aws_apigatewayv2_integration" "get_model" {
  api_id           = aws_apigatewayv2_api.model_loader_api.id
  integration_type = "AWS_PROXY"
  integration_uri  = aws_lambda_function.model_loader_util.invoke_arn
}

###########################################
# Model Loader Lambda Resources
###########################################

# Model Loader Lambda function
resource "aws_lambda_function" "model_loader_util" {
  function_name = "${var.project_name}-${var.environment}-model-loader-util"
  role          = aws_iam_role.lambda_exec.arn
  handler       = "bootstrap"
  runtime       = "provided.al2"
  filename      = "${path.module}/lambda/model-loader-util/model-loader-util.zip"
  source_code_hash = filebase64sha256("${path.module}/lambda/model-loader-util/model-loader-util.zip")

  timeout = 10
  memory_size = 128

  environment {
    variables = {
      model_s3_bucket = var.model_s3_bucket
      api_key_value = var.api_key_value
    }
  }

  tags = local.tags
}

resource "aws_lambda_permission" "model_loader_api_gw" {
  statement_id  = "AllowExecutionFromAPIGateway"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.model_loader_util.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.model_loader_api.execution_arn}/*/*"
}
