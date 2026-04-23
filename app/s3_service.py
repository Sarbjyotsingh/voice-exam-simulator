import boto3
import asyncio
import json
import logging
import datetime
from uuid import uuid4
from app.config import settings

logger = logging.getLogger("app.s3")
s3_client = boto3.client('s3', region_name=settings.aws_region)

def _upload_to_s3_sync(session_id: str, metadata: dict, audio_bytes: bytes):
    timestamp = datetime.datetime.utcnow().strftime("%Y%m%d_%H%M%S")
    base_key = f"sessions/{session_id}/{timestamp}_{str(uuid4())[:8]}"

    try:
        audio_key = f"{base_key}.mp3"
        s3_client.put_object(
            Bucket=settings.s3_bucket_name,
            Key=audio_key,
            Body=audio_bytes,
            ContentType="audio/mpeg"
        )

        metadata["audio_s3_key"] = audio_key
        metadata_key = f"{base_key}.json"
        s3_client.put_object(
            Bucket=settings.s3_bucket_name,
            Key=metadata_key,
            Body=json.dumps(metadata),
            ContentType="application/json"
        )
        logger.info(f"Session data archived: {base_key}")
    except Exception as e:
        logger.error(f"S3 Upload Failed: {e}")

async def archive_session_data(session_id: str, metadata: dict, audio_bytes: bytes):
    """
    Asynchronously uploads session metadata and audio to S3.
    """
    if not settings.s3_bucket_name:
        return

    await asyncio.to_thread(_upload_to_s3_sync, session_id, metadata, audio_bytes)
