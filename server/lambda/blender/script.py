import sys
import os
import boto3
import traceback

try:
    import bpy
except ImportError:
    print("bpy not available outside Blender, but this script is meant to run inside Blender.")

# Parse command-line arguments
args = sys.argv
args = args[args.index("--") + 1:]  # Only args after '--'
params = {}
for arg in args:
    if arg.startswith("--") and "=" in arg:
        k, v = arg[2:].split("=", 1)
        params[k] = v

print("Received parameters from handler:")
for k, v in params.items():
    print(f"{k}: {v}")

print("Environment variables:")
for k, v in os.environ.items():
    print(f"{k}: {v}")

# Extract parameters
from_file_type = params.get("fromFileType")
to_file_type = params.get("toFileType")
model_id = params.get("modelId")
s3_key = params.get("s3Key")
job_type = params.get("jobType")
bucket = os.environ.get("model_s3_bucket")

# S3 client
s3 = boto3.client("s3")

# File paths
input_ext = from_file_type if from_file_type else "blend"
output_ext = to_file_type if to_file_type else "glb"
input_file = f"/tmp/{model_id}.{input_ext}"
output_file = f"/tmp/{model_id}.{output_ext}"

try:
    # Download the input file from S3
    print(f"Downloading {s3_key} from bucket {bucket} to {input_file}")
    s3.download_file(bucket, s3_key, input_file)

    # Load the .blend file
    if from_file_type == "blend":
        print(f"Opening Blender file: {input_file}")
        bpy.ops.wm.open_mainfile(filepath=input_file)
    else:
        raise ValueError(f"Unsupported input file type: {from_file_type}")

    # Export to the desired format
    if to_file_type == "glb":
        print(f"Exporting to GLB: {output_file}")
        bpy.ops.export_scene.gltf(filepath=output_file, export_format='GLB')
    elif to_file_type == "gltf":
        print(f"Exporting to GLTF: {output_file}")
        bpy.ops.export_scene.gltf(filepath=output_file, export_format='GLTF_SEPARATE')
    elif to_file_type == "obj":
        print(f"Exporting to OBJ: {output_file}")
        bpy.ops.export_scene.obj(filepath=output_file)
    else:
        raise ValueError(f"Unsupported output file type: {to_file_type}")

    # Upload the converted file to S3
    output_s3_key = f"converted/{model_id}.{output_ext}"
    print(f"Uploading {output_file} to s3://{bucket}/{output_s3_key}")
    s3.upload_file(output_file, bucket, output_s3_key)

    print("Conversion and upload successful!")

except Exception as e:
    print("Error during Blender conversion or S3 operation:")
    traceback.print_exc()
    sys.exit(1)