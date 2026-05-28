#!/usr/bin/env bash
# deploy-lambda.sh — builds the Go binary for Lambda (Linux/amd64), zips it
# with config files, and updates the Lambda function code.
#
# Usage: ./deploy-lambda.sh <function-name>

set -euo pipefail

FUNCTION_NAME="${1:?Usage: $0 <function-name>}"
BUILD_DIR="$(mktemp -d)"
ZIP_PATH="${BUILD_DIR}/lambda.zip"

echo "Building for linux/amd64..."
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
  go build -o "${BUILD_DIR}/bootstrap" .

echo "Packaging..."
cp temporal.toml "${BUILD_DIR}/temporal.toml"

# If you're bundling certs rather than pulling from Secrets Manager,
# uncomment these lines:
# mkdir -p "${BUILD_DIR}/certs"
# cp certs/client.pem certs/client.key "${BUILD_DIR}/certs/"

cd "${BUILD_DIR}"
zip -j "${ZIP_PATH}" bootstrap temporal.toml

echo "Deploying to Lambda function: ${FUNCTION_NAME}"
aws lambda update-function-code \
  --function-name "${FUNCTION_NAME}" \
  --zip-file "fileb://${ZIP_PATH}"

echo "Done."
rm -rf "${BUILD_DIR}"
