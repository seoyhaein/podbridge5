#!/usr/bin/env bash

# 빌드 단계
echo "Building the image..."
podman build -f Dockerfile.alpine -t alpine_no_executor:test .

# 빌드 성공 여부 확인
if [ $? -eq 0 ]; then
  echo "Build successful. Running the container..."

  # 런 단계
  podman run -it --name alpine_no_executor localhost/alpine_no_executor:test /bin/sh

  # 런 성공 여부 확인
  if [ $? -eq 0 ]; then
    echo "Container ran successfully."
  else
    echo "Failed to run the container."
    exit 2
  fi
else
  echo "Build failed."
  exit 1
fi