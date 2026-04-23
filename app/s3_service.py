import boto3
import asyncio
import json
import logging
from uuid import uuid4
import datetime
from app.config import settings

logger = logging.getLogger("app.s3")

# Boto3 client initialized without hardcoded credentials.
s3_client = boto3.client('s3', region_name=settings.aws_region)

def _upload_to_s3_sync(session_id: str, metadata: dict, audio_bytes: bytes):
    """
    Synchronous botocore/boto3 upload.
    """
    timestamp = datetime.datetime.utcnow().strftime("%Y%m%d_%H%M%S")
    base_key = f"sessions/{session_id}/{timestamp}_{str(uuid4())[:8]}"

    try:
        # 1. Upload audio response
        audio_key = f"{base_key}.mp3"
        s3_client.put_object(
            Bucket=settings.s3_bucket_name,
            Key=audio_key,
            Body=audio_bytes,
            ContentType="audio/mpeg"
        )
        logger.info(f"Audio response uploaded to S3: {audio_key}")

        # 2. Upload metadata
        metadata["audio_s3_key"] = audio_key
        metadata_key = f"{base_key}.json"
        s3_client.put_object(
            Bucket=settings.s3_bucket_name,
            Key=metadata_key,
            Body=json.dumps(metadata),
            ContentType="application/json"
        )
        logger.info(f"Metadata uploaded to S3: {metadata_key}")
    except Exception as e:
        logger.error(f"Failed to upload session data to S3: {str(e)}")

async def archive_session_data(session_id: str, metadata: dict, audio_bytes: bytes):
    """
    Asynchronously uploads session metadata and generated audio bytes to AWS S3.
    """
    if not settings.s3_bucket_name:
        logger.info("S3_BUCKET_NAME not set. Skipping archival.")
        return

    logger.info(f"Starting background archival for session {session_id}...")
    await asyncio.to_thread(_upload_to_s3_sync, session_id, metadata, audio_bytes)
