# syntax=docker/dockerfile:1.4
FROM ubuntu:20.04

ENV DEBIAN_FRONTEND=noninteractive
ENV BLENDER_VERSION=4.0.2

# Install runtime dependencies only
RUN apt-get update && apt-get install -y --no-install-recommends \
    libbz2-dev \
    libegl1 \
    libfontconfig1 \
    libgl1 \
    libglvnd-dev \
    libgtk-3-0 \
    libsm6 \
    libx11-6 \
    libxcursor1 \
    libxext6 \
    libxi6 \
    libxinerama1 \
    libxkbcommon0 \
    libxrandr2 \
    libxrender1 \
    libxxf86vm1 \
    mesa-utils \
    python3 \
    python3-pip \
    wget \
    ca-certificates \
    xz-utils \
 && rm -rf /var/lib/apt/lists/*

 RUN pip3 install boto3

# Install Blender
RUN wget -q https://mirror.clarkson.edu/blender/release/Blender${BLENDER_VERSION%.*}/blender-${BLENDER_VERSION}-linux-x64.tar.xz \
    && tar -xf blender-${BLENDER_VERSION}-linux-x64.tar.xz -C /opt/ \
    && mv /opt/blender-${BLENDER_VERSION}-linux-x64 /opt/blender \
    && ln -s /opt/blender/blender /usr/local/bin/blender \
    && rm blender-${BLENDER_VERSION}-linux-x64.tar.xz

# Add your script
COPY convert.py /app/convert.py
WORKDIR /app

ENTRYPOINT ["python3", "/app/convert.py"]