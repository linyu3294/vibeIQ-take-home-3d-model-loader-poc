
## Server Setup

### AWS Configuration

#### 1. Configure AWS CLI

Add the following to `~/.aws/config`:
```ini
[default]
region = us-east-1
output = json
```

#### 2. Set Up AWS Credentials

1. Create an IAM role in AWS Console
2. Generate access keys
3. Add the following to `~/.aws/credentials`:
```ini
aws_access_key_id = your-secret-id
aws_secret_access_key = your-secret-access-key
```

### Infrastructure Setup

#### S3 Bucket Setup

1. Create a generic S3 bucket in AWS Console:
   - Use default configurations
   - Note down the bucket name for later use

#### Terraform Configuration

1. Copy `terraform.example` to `terraform.tfvars`:
   ```bash
   cp terraform.example terraform.tfvars
   ```

2. Update `terraform.tfvars` with your configuration:
   - Add your S3 bucket name
   - Set other required environment variables
   - Add your secrets

### Deployment

#### Build and Deploy

1. Navigate to the server directory:
   ```bash
   cd server
   ```

2. Run the build script:
   ```bash
   sh ./build.sh
   ```

This will:
- Deploy Lambda functions
- Set up API Gateway routes
- Configure environment variables
- Store API key in AWS Secrets Manager

#### Environment Variables

The following are stored as Lambda environment variables:
- S3 bucket name
- Client domain
- Other configuration values

### Cleanup

To remove all deployed resources:

1. Navigate to the server directory:
   ```bash
   cd server
   ```

2. Run Terraform destroy:
   ```bash
   terraform destroy
   ```

This will remove:
- Lambda functions
- API Gateway routes
- Associated triggers

Note: The S3 bucket must be manually deleted from the AWS Console.
