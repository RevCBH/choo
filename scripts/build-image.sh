#!/bin/bash
set -euo pipefail

# Build the choo Docker image
# Usage: ./scripts/build-image.sh [TAG]

TAG="${1:-latest}"
IMAGE_NAME="choo"

echo "Building ${IMAGE_NAME}:${TAG}..."

docker build \
    -t "${IMAGE_NAME}:${TAG}" \
    -f Dockerfile \
    .

echo "Successfully built ${IMAGE_NAME}:${TAG}"
