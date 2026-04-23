import logging
from groq import AsyncGroq
from app.config import settings

logger = logging.getLogger("app.stt")
client = AsyncGroq(api_key=settings.groq_api_key)

async def transcribe_audio(audio_bytes: bytes) -> str:
    """
    Transcribes raw audio bytes using Groq's whisper-large-v3.
    """
    transcription = await client.audio.transcriptions.create(
        file=("audio.wav", audio_bytes),
        model="whisper-large-v3",
        response_format="text",
        language="de"
    )
    return transcription
