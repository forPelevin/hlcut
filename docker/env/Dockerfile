FROM golang:1

RUN apt-get update && apt-get install -y --no-install-recommends \
    ffmpeg \
    ca-certificates \
    curl \
    git \
    build-essential \
    cmake \
    pkg-config \
    espeak-ng \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /work
