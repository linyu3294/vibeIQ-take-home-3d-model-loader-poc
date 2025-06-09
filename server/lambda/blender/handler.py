import json
import os
import subprocess
import boto3
import logging
import tempfile
from pathlib import Path

# Set up logging
logger = logging.getLogger()
logger.setLevel(logging.INFO)

# Initialize AWS clients
s3 = boto3.client('s3')
sqs = boto3.client('sqs')

def get_conversion_command(input_file: str, output_file: str, from_type: str, to_type: str) -> list:
    """Generate the appropriate Blender command based on file types."""
    if from_type.lower() == 'blend' and to_type.lower() == 'glb':
        return [
            "blender",
            "--background",
            input_file,
            "--python-expr",
            f"import bpy; bpy.ops.export_scene.gltf(filepath='{output_file}', export_format='GLB')"
        ]
    elif from_type.lower() == 'blend' and to_type.lower() == 'gltf':
        return [
            "blender",
            "--background",
            input_file,
            "--python-expr",
            f"import bpy; bpy.ops.export_scene.gltf(filepath='{output_file}', export_format='GLTF_SEPARATE')"
        ]
    else:
        raise ValueError(f"Unsupported conversion from {from_type} to {to_type}")

def process_message(message: dict) -> dict:
    """Process a single SQS message."""
    try:
        # Extract message body
        body = json.loads(message['body'])
        logger.info(f"Processing message: {body}")

        # Extract job details
        job_type = body.get('jobType')
        from_file_type = body.get('fromFileType')
        to_file_type = body.get('toFileType')
        model_id = body.get('modelId')
        s3_key = body.get('s3Key')

        if not all([job_type, from_file_type, to_file_type, model_id, s3_key]):
            raise ValueError("Missing required fields in message")

        if job_type != 'conversion':
            raise ValueError(f"Unsupported job type: {job_type}")

        # Get S3 bucket from environment variable
        model_s3_bucket = os.environ['model_s3_bucket']

        # Create temporary directory for processing
        with tempfile.TemporaryDirectory() as temp_dir:
            # Download input file
            input_file = Path(temp_dir) / f"{model_id}.{from_file_type}"
            logger.info(f"Downloading {s3_key} to {input_file}")
            s3.download_file(model_s3_bucket, s3_key, str(input_file))

            # Prepare output file
            output_file = Path(temp_dir) / f"{model_id}.{to_file_type}"
            
            # Get conversion command
            cmd = get_conversion_command(
                str(input_file),
                str(output_file),
                from_file_type,
                to_file_type
            )

            # Run Blender conversion
            logger.info(f"Running Blender command: {' '.join(cmd)}")
            result = subprocess.run(cmd, capture_output=True, text=True)
            
            if result.returncode != 0:
                raise RuntimeError(f"Blender conversion failed: {result.stderr}")

            # Upload result to S3
            output_s3_key = f"converted/{model_id}.{to_file_type}"
            logger.info(f"Uploading {output_file} to {output_s3_key}")
            s3.upload_file(str(output_file), model_s3_bucket, output_s3_key)

            # Send notification to notification queue
            notification_queue_url = os.environ['NOTIFICATION_QUEUE_URL']
            notification_message = {
                'status': 'completed',
                'modelId': model_id,
                'outputS3Key': output_s3_key,
                'fromFileType': from_file_type,
                'toFileType': to_file_type
            }
            
            sqs.send_message(
                QueueUrl=notification_queue_url,
                MessageBody=json.dumps(notification_message)
            )

            return {
                'status': 'success',
                'modelId': model_id,
                'outputS3Key': output_s3_key
            }

    except Exception as e:
        logger.error(f"Error processing message: {str(e)}")
        # Send error notification
        notification_queue_url = os.environ['NOTIFICATION_QUEUE_URL']
        error_message = {
            'status': 'error',
            'modelId': model_id if 'model_id' in locals() else None,
            'error': str(e)
        }
        
        sqs.send_message(
            QueueUrl=notification_queue_url,
            MessageBody=json.dumps(error_message)
        )
        
        raise

def handler(event, context):
    """AWS Lambda handler function."""
    logger.info(f"Received event: {json.dumps(event)}")
    
    print("ENV VARS:", os.environ)
    print("MODEL S3 BUCKET:", os.environ['model_s3_bucket'])
    print("NOTIFICATION QUEUE URL:", os.environ['NOTIFICATION_QUEUE_URL'])
    
    results = []
    for record in event['Records']:
        try:
            result = process_message(record)
            results.append(result)
        except Exception as e:
            logger.error(f"Failed to process message: {str(e)}")
            results.append({
                'status': 'error',
                'error': str(e)
            })
    
    return {
        'statusCode': 200,
        'body': json.dumps({
            'message': 'Processing complete',
            'results': results
        })
    } 