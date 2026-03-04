#!/usr/bin/env bash
# init-localstack.sh — initializes LocalStack S3 resources on startup.
# Runs automatically via the LocalStack init hook mechanism.

set -euo pipefail

BUCKET_NAME="${BARK_S3_BUCKET:-homebrew-bottles}"
REGION="${AWS_DEFAULT_REGION:-us-east-1}"
ENDPOINT="http://localhost:4566"

echo "==> Creating S3 bucket: ${BUCKET_NAME}"
awslocal s3api create-bucket \
  --bucket "${BUCKET_NAME}" \
  --region "${REGION}"

echo "==> Enabling versioning on ${BUCKET_NAME}"
awslocal s3api put-bucket-versioning \
  --bucket "${BUCKET_NAME}" \
  --versioning-configuration Status=Enabled

echo "==> Bucket ${BUCKET_NAME} ready (versioning enabled)"
