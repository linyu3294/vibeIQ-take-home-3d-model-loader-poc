# convert.py
import bpy
import sys

blend_file = sys.argv[-2]
output_file = sys.argv[-1]

bpy.ops.wm.open_mainfile(filepath=blend_file)
bpy.ops.export_scene.gltf(filepath=output_file, export_format='GLB')