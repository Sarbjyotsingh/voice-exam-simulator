#!/usr/bin/env bash
# Sync static exam assets (manifest, scripts, MP3s) to the public exam bucket.
# Usage: ./exam_generator/upload_to_s3.sh [--dryrun]
set -euo pipefail
cd "$(dirname "$0")/.."
set -a; [ -f .env ] && source .env; set +a
BUCKET="${S3_STATIC_BUCKET:-lingua-stories-exams}"
SRC="exam_generator/upload_package/telc/"

# JSON: short cache so script edits propagate; MP3: immutable long cache.
aws s3 sync "$SRC" "s3://$BUCKET/telc/" --exclude "*" --include "*.json" \
  --content-type application/json --cache-control "public, max-age=300" "$@"
aws s3 sync "$SRC" "s3://$BUCKET/telc/" --exclude "*" --include "*.mp3" \
  --content-type audio/mpeg --cache-control "public, max-age=31536000, immutable" "$@"
