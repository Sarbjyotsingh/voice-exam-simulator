import logging
import json
from groq import AsyncGroq
from app.config import settings

logger = logging.getLogger("app.stt")
client = AsyncGroq(api_key=settings.groq_api_key)

async def transcribe_audio(audio_bytes: bytes) -> str:
    """
    Transcribes raw audio bytes using Groq's whisper-large-v3.
    """
    # VERBOSE DEBUG: Print exactly what is being sent to STT
    logger.info("================ STT DEBUG START ================")
    logger.info(f"Model: whisper-large-v3")
    logger.info(f"Audio Size: {len(audio_bytes)} bytes")
    logger.info(f"Response Format: text")
    logger.info("================= STT DEBUG END =================")

    transcription = await client.audio.transcriptions.create(
        file=("audio.wav", audio_bytes),
        model="whisper-large-v3",
        response_format="text",
        language="de"
    )
    logger.info("Transcription successful.")
    return transcription
