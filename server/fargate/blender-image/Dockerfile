# Use a minimal Ubuntu base
FROM ubuntu:22.04

# Install dependencies
RUN apt-get update && apt-get install -y \
    wget \
    ca-certificates \
    libgl1 \
    libx11-6 \
    xvfb \
    python3 \
    xz-utils \  
    && apt-get clean

# Set Blender version
ENV BLENDER_VERSION=4.0.2

# Download and extract Blender
RUN wget https://download.blender.org/release/Blender${BLENDER_VERSION%.*}/blender-${BLENDER_VERSION}-linux-x64.tar.xz && \
    tar -xf blender-${BLENDER_VERSION}-linux-x64.tar.xz && \
    mv blender-${BLENDER_VERSION}-linux-x64 /opt/blender && \
    ln -s /opt/blender/blender /usr/local/bin/blender

# Set default workdir
WORKDIR /workspace

# Default command (can be overridden by ECS)
ENTRYPOINT ["blender"]
