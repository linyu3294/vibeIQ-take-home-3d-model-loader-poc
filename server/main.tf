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

###########################################
# Model Loader S3 Resources
###########################################
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

###########################################
# Model Loader Lambda Resources
###########################################
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
      sqs_queue_url = aws_sqs_queue.blender_jobs.url
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
  role       = aws_iam_role.lambda_exec.name
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
  role          = aws_iam_role.lambda_exec.arn
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
  role          = aws_iam_role.lambda_exec.arn
  handler       = "bootstrap"
  runtime       = "provided.al2"
  filename      = "${path.module}/lambda/websocket-connect/websocket-connect.zip"
  source_code_hash = filebase64sha256("${path.module}/lambda/websocket-connect/websocket-connect.zip")

  environment {
    variables = {
      CONNECTIONS_TABLE = aws_dynamodb_table.websocket_connections.name
      api_key_value = var.api_key_value
    }
  }

  tags = local.tags
}

resource "aws_lambda_function" "websocket_disconnect" {
  function_name = "${var.project_name}-${var.environment}-websocket-disconnect"
  role          = aws_iam_role.lambda_exec.arn
  handler       = "bootstrap"
  runtime       = "provided.al2"
  filename      = "${path.module}/lambda/websocket-disconnect/websocket-disconnect.zip"
  source_code_hash = filebase64sha256("${path.module}/lambda/websocket-disconnect/websocket-disconnect.zip")

  environment {
    variables = {
      CONNECTIONS_TABLE = aws_dynamodb_table.websocket_connections.name
      api_key_value = var.api_key_value
    }
  }

  tags = local.tags
}

###########################################
# AWS ECS Fargate Resources
###########################################

resource "aws_vpc" "blender" {
  cidr_block           = "10.0.0.0/16"
  enable_dns_support   = true
  enable_dns_hostnames = true
  tags = {
    Name = "${var.project_name}-${var.environment}-vpc"
  }
}

resource "aws_subnet" "blender_public" {
  vpc_id                  = aws_vpc.blender.id
  cidr_block              = "10.0.1.0/24"
  map_public_ip_on_launch = true
  availability_zone       = "us-east-1a"
  tags = {
    Name = "${var.project_name}-${var.environment}-public-subnet"
  }
}

resource "aws_internet_gateway" "blender_igw" {
  vpc_id = aws_vpc.blender.id
  tags = {
    Name = "${var.project_name}-${var.environment}-igw"
  }
}

resource "aws_route_table" "blender_public" {
  vpc_id = aws_vpc.blender.id
  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.blender_igw.id
  }
  tags = {
    Name = "${var.project_name}-${var.environment}-public-rt"
  }
}

resource "aws_route_table_association" "blender_public_assoc" {
  subnet_id      = aws_subnet.blender_public.id
  route_table_id = aws_route_table.blender_public.id
}

resource "aws_security_group" "blender_ecs_sg" {
  name        = "${var.project_name}-${var.environment}-ecs-blender-sg"
  description = "Allow Blender VNC and Web UI"
  vpc_id      = aws_vpc.blender.id

  ingress {
    description = "Allow VNC"
    from_port   = 5900
    to_port     = 5900
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    description = "Allow Blender Web UI"
    from_port   = 3000
    to_port     = 3000
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "${var.project_name}-${var.environment}-ecs-blender-sg"
  }
}

resource "aws_ecs_cluster" "blender" {
  name = "${var.project_name}-${var.environment}-blender-cluster"
}

resource "aws_ecs_task_definition" "blender" {
  family                   = "${var.project_name}-${var.environment}-blender-task"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = "1024" # 1 vCPU
  memory                   = "2048" # 2 GB
  execution_role_arn       = aws_iam_role.ecs_task_execution.arn
  task_role_arn            = aws_iam_role.ecs_task_execution.arn

  container_definitions = jsonencode([
    {
      name      = "blender"
      image     = var.blender_ecr_image
      essential = true
      command = []
      logConfiguration = {
        logDriver = "awslogs"
        options = {
          awslogs-group         = "/ecs/blender"
          awslogs-region        = "us-east-1"
          awslogs-stream-prefix = "blender"
        }
      }
      portMappings = [
        {
          containerPort = 3000
          hostPort      = 3000
          protocol      = "tcp"
        },
        {
          containerPort = 5900
          hostPort      = 5900
          protocol      = "tcp"
        }
      ]
      environment = [
        {
          name  = "AWS_REGION"
          value = var.region
        },
        {
          name  = "blender_jobs_queue_url"
          value = aws_sqs_queue.blender_jobs.url
        },
        {
          name  = "notification_queue_url"
          value = aws_sqs_queue.notification_queue.url
        }
      ]
    }
  ])
}

resource "aws_ecs_service" "blender" {
  name            = "${var.project_name}-${var.environment}-blender-service"
  cluster         = aws_ecs_cluster.blender.id
  task_definition = aws_ecs_task_definition.blender.arn
  launch_type     = "FARGATE"
  desired_count   = 1

  network_configuration {
    subnets          = [aws_subnet.blender_public.id]
    security_groups  = [aws_security_group.blender_ecs_sg.id]
    assign_public_ip = true
  }

  depends_on = [aws_iam_role_policy_attachment.ecs_task_execution]
}

resource "aws_cloudwatch_log_group" "blender" {
  name              = "/ecs/blender"
  retention_in_days = 7
}

resource "aws_iam_role" "ecs_task_execution" {
  name = "3d-model-loader-${var.environment}-ecs-task-execution-role"
  assume_role_policy = jsonencode({
    Version = "2012-10-17",
    Statement = [{
      Action = "sts:AssumeRole"
      Effect = "Allow"
      Principal = {
        Service = "ecs-tasks.amazonaws.com"
      }
    }]
  })
}

resource "aws_iam_role_policy_attachment" "ecs_task_execution" {
  role       = aws_iam_role.ecs_task_execution.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

###########################################
# SQS Queue for Blender Jobs
###########################################
resource "aws_sqs_queue" "blender_jobs" {
  name = "${var.project_name}-${var.environment}-blender-jobs"
  visibility_timeout_seconds = 900  # 15 minutes, matching Fargate task timeout
  message_retention_seconds = 86400 # 1 day
  delay_seconds = 0
  receive_wait_time_seconds = 20    # Enable long polling
  tags = local.tags
}

# IAM policy for Lambda to send messages to SQS
resource "aws_iam_role_policy" "lambda_sqs_policy" {
  name = "${var.project_name}-${var.environment}-lambda-sqs-policy"
  role = aws_iam_role.lambda_exec.id

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

# IAM policy for ECS task to receive messages from SQS
resource "aws_iam_role_policy" "ecs_sqs_policy" {
  name = "${var.project_name}-${var.environment}-ecs-sqs-policy"
  role = aws_iam_role.ecs_task_execution.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "sqs:ReceiveMessage",
          "sqs:DeleteMessage",
          "sqs:GetQueueUrl",
          "sqs:ChangeMessageVisibility"
        ]
        Resource = aws_sqs_queue.blender_jobs.arn
      }
    ]
  })
}

###########################################
# Notification SQS Queue
###########################################
resource "aws_sqs_queue" "notification_queue" {
  name = "${var.project_name}-${var.environment}-notification-queue"
  visibility_timeout_seconds = 30
  message_retention_seconds = 86400 # 1 day
  delay_seconds = 0
  receive_wait_time_seconds = 20

  tags = local.tags
}

# IAM policy for Fargate to send messages to notification queue
resource "aws_iam_role_policy" "ecs_notification_policy" {
  name = "${var.project_name}-${var.environment}-ecs-notification-policy"
  role = aws_iam_role.ecs_task_execution.id

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

###########################################
# Notification Lambda
###########################################
resource "aws_lambda_function" "notification" {
  function_name = "${var.project_name}-${var.environment}-notification"
  role          = aws_iam_role.lambda_exec.arn
  handler       = "bootstrap"
  runtime       = "provided.al2"
  filename      = "${path.module}/lambda/notification/notification.zip"
  source_code_hash = filebase64sha256("${path.module}/lambda/notification/notification.zip")

  environment {
    variables = {
      CONNECTIONS_TABLE = aws_dynamodb_table.websocket_connections.name
      WEBSOCKET_API_ENDPOINT = "${aws_apigatewayv2_api.websocket_api.api_endpoint}/${aws_apigatewayv2_stage.websocket_api_stage.name}"
    }
  }

  tags = local.tags
}

# IAM policy for notification Lambda to access DynamoDB and SQS
resource "aws_iam_role_policy" "notification_lambda_policy" {
  name = "${var.project_name}-${var.environment}-notification-lambda-policy"
  role = aws_iam_role.lambda_exec.id

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
}
