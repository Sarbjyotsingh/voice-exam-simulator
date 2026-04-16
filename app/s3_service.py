import boto3
import asyncio
import json
from uuid import uuid4
import datetime
from app.config import settings

# Boto3 client initialized without hardcoded credentials.
# On AWS Fargate, this automatically uses the IAM Task Role attached to the container.
s3_client = boto3.client('s3', region_name=settings.aws_region)

def _upload_to_s3_sync(session_id: str, metadata: dict, audio_bytes: bytes):
    """
    Synchronous botocore/boto3 upload.
    Will be passed to a ThreadPool via asyncio.to_thread to make it non-blocking.
    """
    timestamp = datetime.datetime.utcnow().strftime("%Y%m%d_%H%M%S")
    base_key = f"sessions/{session_id}/{timestamp}_{str(uuid4())[:8]}"

    # 1. Upload audio response
    audio_key = f"{base_key}.mp3"
    s3_client.put_object(
        Bucket=settings.s3_bucket_name,
        Key=audio_key,
        Body=audio_bytes,
        ContentType="audio/mpeg"
    )

    # 2. Upload metadata
    metadata["audio_s3_key"] = audio_key
    metadata_key = f"{base_key}.json"
    s3_client.put_object(
        Bucket=settings.s3_bucket_name,
        Key=metadata_key,
        Body=json.dumps(metadata),
        ContentType="application/json"
    )

async def archive_session_data(session_id: str, metadata: dict, audio_bytes: bytes):
    """
    Asynchronously uploads session metadata and generated audio bytes to AWS S3.
    Uses asyncio.to_thread to wrap synchronous boto3 calls so that the main event loop
    handling WebSocket streams is never blocked.
    """
    if not settings.s3_bucket_name:
        return # Skip if no bucket configured

    # Fire and forget execution in a thread pool
    await asyncio.to_thread(_upload_to_s3_sync, session_id, metadata, audio_bytes)
