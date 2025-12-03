#!/bin/bash
set -e

export DOCKER_BUILDKIT=1

IMAGE="eu-frankfurt-1.ocir.io/axobgaeopgxr/tsf/maternify-atermes-jwt:latest"

docker buildx create --use --name arm-builder || true
docker buildx inspect --bootstrap

docker buildx build \
  --platform linux/arm64 \
  -t $IMAGE \
  --push .