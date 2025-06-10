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
resource "aws_iam_role" "lambda_app_exec" {
  name = "${var.project_name}-${var.environment}-lambda-app-exec-role"

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

resource "aws_iam_role_policy_attachment" "lambda_app_policy" {
  role       = aws_iam_role.lambda_app_exec.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

###########################################
# Model Loader S3 Resources
###########################################
resource "aws_iam_role_policy" "lambda_app_s3_policy" {
  name = "${var.project_name}-${var.environment}-lambda-app-s3-policy"
  role = aws_iam_role.lambda_app_exec.id

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

###########################################
# Model Loader HTTP API Gateway Resources
###########################################
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

# Add POST /3d-model route and integration
resource "aws_apigatewayv2_route" "post_model" {
  api_id    = aws_apigatewayv2_api.model_loader_api.id
  route_key = "POST /3d-model"
  target    = "integrations/${aws_apigatewayv2_integration.post_model.id}"
  authorization_type = "NONE"
}

resource "aws_apigatewayv2_integration" "post_model" {
  api_id           = aws_apigatewayv2_api.model_loader_api.id
  integration_type = "AWS_PROXY"
  integration_uri  = aws_lambda_function.model_loader_util.invoke_arn
}

# Add PUT /3d-model/{id} route and integration for direct S3 upload via Lambda
resource "aws_apigatewayv2_route" "put_model" {
  api_id    = aws_apigatewayv2_api.model_loader_api.id
  route_key = "PUT /3d-model/{id}"
  target    = "integrations/${aws_apigatewayv2_integration.put_model.id}"
  authorization_type = "NONE"
}

resource "aws_apigatewayv2_integration" "put_model" {
  api_id           = aws_apigatewayv2_api.model_loader_api.id
  integration_type = "AWS_PROXY"
  integration_uri  = aws_lambda_function.model_loader_util.invoke_arn
}

###########################################
# Model Loader Lambda Resources
###########################################
resource "aws_lambda_function" "model_loader_util" {
  function_name = "${var.project_name}-${var.environment}-model-loader-util"
  role          = aws_iam_role.lambda_app_exec.arn
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
      blender_jobs_queue_url = aws_sqs_queue.blender_jobs.url
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

###########################################
# AWS API Gateway WebSocket Resources
###########################################
resource "aws_apigatewayv2_api" "websocket_api" {
  name          = "${var.project_name}-${var.environment}-websocket-api"
  protocol_type = "WEBSOCKET"
  route_selection_expression = "$request.body.action"
}

resource "aws_apigatewayv2_stage" "websocket_api_stage" {
  api_id      = aws_apigatewayv2_api.websocket_api.id
  name        = "prod"
  auto_deploy = true
}


###########################################
# AWS API Gateway WebSocket Integration Resources
###########################################

resource "aws_apigatewayv2_integration" "websocket_default" {
  api_id           = aws_apigatewayv2_api.websocket_api.id
  integration_type = "AWS_PROXY"
  integration_uri  = aws_lambda_function.websocket_default.invoke_arn
}

resource "aws_apigatewayv2_integration" "websocket_connect" {
  api_id           = aws_apigatewayv2_api.websocket_api.id
  integration_type = "AWS_PROXY"
  integration_uri  = aws_lambda_function.websocket_connect.invoke_arn
}

resource "aws_apigatewayv2_integration" "websocket_disconnect" {
  api_id           = aws_apigatewayv2_api.websocket_api.id
  integration_type = "AWS_PROXY"
  integration_uri  = aws_lambda_function.websocket_disconnect.invoke_arn
}

###########################################
# AWS API Gateway WebSocket Routes Resources
###########################################

resource "aws_apigatewayv2_route" "websocket_connect_route" {
  api_id    = aws_apigatewayv2_api.websocket_api.id
  route_key = "$connect"
  target    = "integrations/${aws_apigatewayv2_integration.websocket_connect.id}"
}

resource "aws_apigatewayv2_route" "websocket_disconnect_route" {
  api_id    = aws_apigatewayv2_api.websocket_api.id
  route_key = "$disconnect"
  target    = "integrations/${aws_apigatewayv2_integration.websocket_disconnect.id}"
}

resource "aws_apigatewayv2_route" "websocket_default_route" {
  api_id    = aws_apigatewayv2_api.websocket_api.id
  route_key = "$default"
  target    = "integrations/${aws_apigatewayv2_integration.websocket_default.id}"
}


###########################################
# AWS API Gateway WebSocket Management DynamoDB Resources
###########################################
resource "aws_dynamodb_table" "websocket_connections" {
  name         = "${var.project_name}-${var.environment}-websocket-connections"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "connectionId"

  attribute {
    name = "connectionId"
    type = "S"
  }

  tags = local.tags
}

resource "aws_iam_role_policy_attachment" "connect_lambda_dynamodb" {
  role       = aws_iam_role.lambda_app_exec.name
  policy_arn = aws_iam_policy.dynamodb_access.arn
}


resource "aws_iam_policy" "dynamodb_access" {
  name        = "${var.project_name}-${var.environment}-dynamodb-access"
  description = "Allow Lambda to access DynamoDB for WebSocket connection management."

  policy = jsonencode({
    Version = "2012-10-17",
    Statement = [
      {
        Effect = "Allow",
        Action = [
          "dynamodb:PutItem",
          "dynamodb:DeleteItem"
        ],
        Resource = aws_dynamodb_table.websocket_connections.arn
      }
    ]
  })
}

###########################################
# AWS API Gateway WebSocket Permission Resources
###########################################
resource "aws_lambda_permission" "websocket_connect_permission" {
  statement_id  = "AllowExecutionFromAPIGatewayConnect"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.websocket_connect.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.websocket_api.execution_arn}/*/*"
}

resource "aws_lambda_permission" "websocket_disconnect_permission" {
  statement_id  = "AllowExecutionFromAPIGatewayDisconnect"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.websocket_disconnect.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.websocket_api.execution_arn}/*/*"
}

resource "aws_lambda_permission" "websocket_default_permission" {
  statement_id  = "AllowExecutionFromAPIGatewayDefault"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.websocket_default.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.websocket_api.execution_arn}/*/*"
}

###########################################
# AWS API Gateway WebSocket Management Lambda Resources
###########################################
resource "aws_lambda_function" "websocket_default" {
  function_name = "${var.project_name}-${var.environment}-websocket-default"
  role          = aws_iam_role.lambda_app_exec.arn
  handler       = "bootstrap"
  runtime       = "provided.al2"
  filename      = "${path.module}/lambda/websocket-default/websocket-default.zip"
  source_code_hash = filebase64sha256("${path.module}/lambda/websocket-default/websocket-default.zip")

  environment {
    variables = {
      # Add any needed environment variables here
    }
  }

  tags = local.tags
}

resource "aws_lambda_function" "websocket_connect" {
  function_name = "${var.project_name}-${var.environment}-websocket-connect"
  role          = aws_iam_role.lambda_app_exec.arn
  handler       = "bootstrap"
  runtime       = "provided.al2"
  filename      = "${path.module}/lambda/websocket-connect/websocket-connect.zip"
  source_code_hash = filebase64sha256("${path.module}/lambda/websocket-connect/websocket-connect.zip")

  environment {
    variables = {
      connections_table = aws_dynamodb_table.websocket_connections.name
      api_key_value = var.api_key_value
    }
  }

  tags = local.tags
}

resource "aws_lambda_function" "websocket_disconnect" {
  function_name = "${var.project_name}-${var.environment}-websocket-disconnect"
  role          = aws_iam_role.lambda_app_exec.arn
  handler       = "bootstrap"
  runtime       = "provided.al2"
  filename      = "${path.module}/lambda/websocket-disconnect/websocket-disconnect.zip"
  source_code_hash = filebase64sha256("${path.module}/lambda/websocket-disconnect/websocket-disconnect.zip")

  environment {
    variables = {
      connections_table = aws_dynamodb_table.websocket_connections.name
      api_key_value = var.api_key_value
    }
  }

  tags = local.tags
}

###########################################
# AWS Blender Jobs SQS Resources
###########################################

resource "aws_sqs_queue" "blender_jobs" {
  name = "${var.project_name}-${var.environment}-blender-jobs"
  visibility_timeout_seconds = 900
  message_retention_seconds  = 86400
  delay_seconds              = 0
  receive_wait_time_seconds  = 20
  tags = local.tags
}

# Lambda policy to send to SQS
resource "aws_iam_role_policy" "lambda_sqs_policy" {
  name = "${var.project_name}-${var.environment}-lambda-sqs-policy"
  role = aws_iam_role.lambda_app_exec.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "sqs:SendMessage",
          "sqs:GetQueueUrl"
        ]
        Resource = aws_sqs_queue.blender_jobs.arn
      }
    ]
  })
}

###########################################
# AWS Blender Lambda Resources
###########################################

resource "aws_lambda_function" "blender" {
  function_name = "${var.project_name}-${var.environment}-blender-lambda"
  role          = aws_iam_role.lambda_blender_exec.arn # Reuse the ECS task execution role for permissions
  package_type  = "Image"
  image_uri     = var.blender_ecr_image
  timeout       = 300 # 5 minutes
  memory_size   = 4096 # 4GB

  environment {
    variables = {
      model_s3_bucket                = var.model_s3_bucket
      blender_jobs_queue_url   = aws_sqs_queue.blender_jobs.url
      notification_queue_url   = aws_sqs_queue.notification_queue.url
    }
  }
}

resource "aws_lambda_event_source_mapping" "blender_jobs_trigger" {
  event_source_arn = aws_sqs_queue.blender_jobs.arn
  function_name    = aws_lambda_function.blender.arn
  batch_size       = 1
  enabled          = true
}


# Add ECS Task Execution Role for Blender Lambda (used as Lambda execution role)
resource "aws_iam_role" "lambda_blender_exec" {
  name = "3d-model-loader-${var.environment}-lambda-blender-exec-role"
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
}

resource "aws_iam_role_policy_attachment" "lambda_blender_policy" {
  role       = aws_iam_role.lambda_blender_exec.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

# Add S3 and SQS permissions for Blender Lambda (lambda_blender_exec role)
resource "aws_iam_role_policy" "lambda_blender_s3_sqs_policy" {
  name = "${var.project_name}-${var.environment}-lambda-blender-s3-sqs-policy"
  role = aws_iam_role.lambda_blender_exec.id

  policy = jsonencode({
    Version = "2012-10-17",
    Statement = [
      {
        Effect = "Allow",
        Action = [
          "s3:GetObject",
          "s3:PutObject",
          "s3:DeleteObject"
        ],
        Resource = "arn:aws:s3:::${var.model_s3_bucket}/*"
      },
      {
        Effect = "Allow",
        Action = [
          "sqs:SendMessage",
          "sqs:GetQueueUrl"
        ],
        Resource = aws_sqs_queue.notification_queue.arn
      },
      {
        Effect = "Allow",
        Action = [
          "sqs:ReceiveMessage",
          "sqs:DeleteMessage",
          "sqs:GetQueueAttributes"
        ],
        Resource = aws_sqs_queue.blender_jobs.arn
      }
    ]
  })
}

###########################################
# AWS Notification SQS Resources
###########################################

resource "aws_sqs_queue" "notification_queue" {
  name = "${var.project_name}-${var.environment}-notification-queue"
  visibility_timeout_seconds = 30
  message_retention_seconds = 86400 # 1 day
  delay_seconds = 0
  receive_wait_time_seconds = 20
  tags = local.tags
}

###########################################
# Notification Lambda
###########################################

resource "aws_lambda_function" "notification" {
  function_name = "${var.project_name}-${var.environment}-notification"
  role          = aws_iam_role.lambda_app_exec.arn
  handler       = "bootstrap"
  runtime       = "provided.al2"
  filename      = "${path.module}/lambda/notification/notification.zip"
  source_code_hash = filebase64sha256("${path.module}/lambda/notification/notification.zip")
  depends_on = [aws_dynamodb_table.websocket_connections]
  
  environment {
    variables = {
      connections_table = aws_dynamodb_table.websocket_connections.name
      websocket_api_endpoint = "${aws_apigatewayv2_api.websocket_api.api_endpoint}/${aws_apigatewayv2_stage.websocket_api_stage.name}"
    }
  }

  tags = local.tags
}

# IAM policy for notification Lambda to access DynamoDB and SQS
resource "aws_iam_role_policy" "notification_lambda_policy" {
  name = "${var.project_name}-${var.environment}-notification-lambda-policy"
  role = aws_iam_role.lambda_app_exec.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "dynamodb:Scan"
        ]
        Resource = aws_dynamodb_table.websocket_connections.arn
      },
      {
        Effect = "Allow"
        Action = [
          "sqs:ReceiveMessage",
          "sqs:DeleteMessage",
          "sqs:GetQueueUrl",
          "sqs:GetQueueAttributes"
        ]
        Resource = aws_sqs_queue.notification_queue.arn
      }
    ]
  })
}

# SQS trigger for notification Lambda
resource "aws_lambda_event_source_mapping" "notification_trigger" {
  event_source_arn = aws_sqs_queue.notification_queue.arn
  function_name    = aws_lambda_function.notification.arn
  batch_size       = 1
  enabled          = true
}

# Add IAM policy for Blender Lambda to send to notification queue
resource "aws_iam_role_policy" "blender_notification_policy" {
  name = "${var.project_name}-${var.environment}-blender-notification-policy"
  role = aws_iam_role.lambda_blender_exec.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "sqs:SendMessage",
          "sqs:GetQueueUrl"
        ]
        Resource = aws_sqs_queue.notification_queue.arn
      }
    ]
  })
}
