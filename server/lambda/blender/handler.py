import json
import os
import subprocess
import logging
import boto3

logger = logging.getLogger()
logger.setLevel(logging.INFO)

def send_notification(queue_url, message):
    sqs = boto3.client('sqs')
    sqs.send_message(QueueUrl=queue_url, MessageBody=json.dumps(message))
    logger.info(f"Notification sent to SQS: {message}")

def handler(event, context):
    notification_queue_url = os.environ.get('notification_queue_url')
    bucket = os.environ.get('model_s3_bucket')
    if not notification_queue_url:
        raise RuntimeError("notification_queue_url environment variable is not set")
    if not bucket:
        raise RuntimeError("model_s3_bucket environment variable is not set")

    logger.info(f"Received event: {json.dumps(event)}")
    for record in event['Records']:
        try:
            body = json.loads(record['body'])
            job_type = body.get('jobType')
            job_id = body.get('jobId')
            job_status = body.get('jobStatus')
            from_file_type = body.get('fromFileType')
            to_file_type = body.get('toFileType')
            model_id = body.get('modelId')
            s3_key = body.get('s3Key')
            connection_id = body.get('connectionId')

            cmd = [
                "blender", "-b", "-P", "script.py", "--",
                f"--fromFileType={from_file_type}",
                f"--toFileType={to_file_type}",
                f"--modelId={model_id}",
                f"--s3Key={s3_key}",
                f"--jobType={job_type}"
            ]
            logger.info(f"Running Blender command: {' '.join(cmd)}")
            result = subprocess.run(cmd, capture_output=True, text=True, env=os.environ.copy())
            logger.info(f"Blender stdout: {result.stdout}")
            logger.info(f"Blender stderr: {result.stderr}")
            if result.returncode != 0:
                raise RuntimeError(f"Blender conversion failed: {result.stderr}")

            output_file = None
            for line in result.stdout.splitlines():
                if line.startswith("OUTPUT_FILE="):
                    output_file = line.split("=", 1)[1].strip()
                    break
            if not output_file or not os.path.exists(output_file):
                raise RuntimeError("Output file not found after Blender conversion.")

            s3 = boto3.client('s3')
            new_s3_key = f"{to_file_type}/{os.path.basename(output_file)}"
            logger.info(f"Uploading {output_file} to s3://{bucket}/{new_s3_key}")
            s3.upload_file(output_file, bucket, new_s3_key)

            notification = {
                "connectionId": connection_id,
                "jobType": job_type,
                "jobId": job_id,
                "jobStatus": "completed",
                "fromFileType": from_file_type,
                "toFileType": to_file_type,
                "modelId": model_id,
                "s3Key": s3_key,
                "newS3Key": new_s3_key,
            }
            send_notification(notification_queue_url, notification)

        except Exception as e:
            logger.error(f"Error processing record: {str(e)}")
            error_notification = {
                "connectionId": connection_id,
                "jobType": body.get('jobType') if 'body' in locals() else None,
                "jobId": body.get('jobId') if 'body' in locals() else None,
                "jobStatus": "failed",
                "fromFileType": body.get('fromFileType') if 'body' in locals() else None,
                "toFileType": body.get('toFileType') if 'body' in locals() else None,
                "modelId": body.get('modelId') if 'body' in locals() else None,
                "s3Key": body.get('s3Key') if 'body' in locals() else None,
                "newS3Key": new_s3_key if 'new_s3_key' in locals() else None,
                "error": str(e)
            }
            send_notification(notification_queue_url, error_notification)

    return {
        "statusCode": 200,
        "body": json.dumps({"message": "Processing complete"})
    }