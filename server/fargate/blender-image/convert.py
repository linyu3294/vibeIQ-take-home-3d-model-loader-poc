import boto3
import os
import json
import subprocess
import sys

sqs = boto3.client('sqs')
s3 = boto3.client('s3')

blender_jobs_queue_url = os.environ['blender_jobs_queue_url']
notification_queue_url = os.environ['notification_queue_url']

BLENDER_SCRIPT_PATH = '/app/blender_convert_script.py'  # This script must exist in the image


def process_job(job):
    input_bucket = job['input_bucket']
    input_key = job['input_key']
    output_bucket = job['output_bucket']
    output_key = job['output_key']
    job_id = job['job_id']

    try:
        # Download .blend file from S3
        s3.download_file(input_bucket, input_key, '/tmp/model.blend')

        # Call Blender to convert .blend to .glb
        result = subprocess.run([
            'blender', '--background', '--python', BLENDER_SCRIPT_PATH, '--',
            '/tmp/model.blend', '/tmp/model.glb'
        ], capture_output=True, text=True)

        if result.returncode != 0:
            raise Exception(f'Blender conversion failed: {result.stderr}')

        # Upload .glb to S3
        s3.upload_file('/tmp/model.glb', output_bucket, output_key)

        # Send success notification
        notification = {
            'job_id': job_id,
            'status': 'success',
            'glb_s3_path': f's3://{output_bucket}/{output_key}'
        }
        sqs.send_message(
            QueueUrl=notification_queue_url,
            MessageBody=json.dumps(notification)
        )
    except Exception as e:
        # Send failure notification
        notification = {
            'job_id': job_id,
            'status': 'failure',
            'error': str(e)
        }
        sqs.send_message(
            QueueUrl=notification_queue_url,
            MessageBody=json.dumps(notification)
        )
        print(f'Error processing job {job_id}: {e}', file=sys.stderr)


def main():
    while True:
        response = sqs.receive_message(
            QueueUrl=blender_jobs_queue_url,
            MaxNumberOfMessages=1,
            WaitTimeSeconds=20
        )
        messages = response.get('Messages', [])
        for message in messages:
            job = json.loads(message['Body'])
            process_job(job)
            # Delete message from queue after processing
            sqs.delete_message(
                QueueUrl=blender_jobs_queue_url,
                ReceiptHandle=message['ReceiptHandle']
            )

if __name__ == "__main__":
    main()