variable "region" {
  description = "AWS region to deploy resources"
  type        = string
  default     = "us-east-1"
}

variable "project_name" {
  description = "Prefix for all resources"
  type        = string
  default     = "vibeIQ-take-home-3d-model-loader-poc"
}

variable "environment" {
  description = "Environment (e.g., dev, prod)"
  type        = string
  default     = "prod"
}

variable "api_stage_name" {
  description = "API Gateway stage name"
  type        = string
  default     = "v1"
}

variable "client_domain" {
  description = "Allowed client domain for CORS"
  type        = string
}

variable "model_s3_bucket" {
  description = "S3 bucket for Model files (GLB files)"
  type        = string
}

variable "api_key_value" {
  description = "API key value for API Gateway"
  type        = string
  sensitive   = true
}

variable "allowed_origins" {
  description = "List of allowed origins for CORS"
  type        = list(string)
}

variable "blender_ecr_image" {
  description = "ECR image URI for the Blender Lambda container"
  type        = string
}