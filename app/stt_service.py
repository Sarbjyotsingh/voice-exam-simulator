from groq import AsyncGroq
from app.config import settings

client = AsyncGroq(api_key=settings.groq_api_key)

async def transcribe_audio(audio_bytes: bytes) -> str:
    """
    Transcribes raw audio bytes using Groq's whisper-large-v3.
    Passes bytes directly to avoid Disk I/O, minimizing TTFB.
    """
    transcription = await client.audio.transcriptions.create(
        file=("audio.wav", audio_bytes),
        model="whisper-large-v3",
        response_format="text"
    )
    return transcription
