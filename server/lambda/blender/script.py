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
bucket = os.environ.get("model_s3_bucket")
s3_key = params.get("s3Key")

from_file_type = params.get("fromFileType")
to_file_type = params.get("toFileType")
model_id = params.get("modelId")


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
        bpy.ops.export_scene.gltf(
            filepath=output_file,
            export_format='GLB',
            export_texcoords=True,
            export_normals=True,
            export_yup=True
            )
    elif to_file_type == "gltf":
        bpy.ops.export_scene.gltf(
            filepath=output_file,
            export_format='GLTF_SEPARATE',
            export_texcoords=True,
            export_normals=True,
            export_yup=True
        )
    elif to_file_type == "obj":
        bpy.ops.export_scene.obj(
            filepath=output_file,
            use_materials=True
        )
    elif to_file_type == "fbx":
        bpy.ops.export_scene.fbx(
            filepath=output_file,
            use_selection=False,
            apply_unit_scale=True,
            bake_space_transform=True
        )
    elif to_file_type == "usd":
        bpy.ops.wm.usd_export(filepath=output_file)
    elif to_file_type == "usdz":
        # Export as .usd first
        intermediate_usd = output_file.replace(".usdz", ".usd")
        bpy.ops.wm.usd_export(filepath=intermediate_usd)
    else:
        raise ValueError(f"Unsupported output file type: {to_file_type}")

    # At the end, print the output file path for handler.py to parse
    print(f"OUTPUT_FILE={output_file}")
    print("Conversion successful!")

except Exception as e:
    print("Error during Blender conversion:")
    traceback.print_exc()
    sys.exit(1)