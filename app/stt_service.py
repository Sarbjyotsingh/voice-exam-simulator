import logging
import re
from groq import AsyncGroq
from app.config import settings

logger = logging.getLogger("app.stt")
client = AsyncGroq(api_key=settings.groq_api_key)

# Common Whisper hallucinations, especially for German/Silence
WHISPER_HALLUCINATIONS = [
    "Vielen Dank.",
    "Vielen Dank für das Zuschauen.",
    "Danke fürs Zuschauen.",
    "Untertitelung im Auftrag des ZDF",
    "Untertitelung von TV-M",
    "Untertitelung der ARD",
    "Danke.",
    "Vielen dank.",
    "Veilen danke",  # User reported variant
    "Vielen Dank!",
    "Danke!",
]

def _is_hallucination(text: str) -> bool:
    """
    Checks if the transcribed text is a common Whisper hallucination.
    """
    normalized = re.sub(r'[^\w\s]', '', text).strip().lower()
    if not normalized:
        return True
        
    for h in WHISPER_HALLUCINATIONS:
        h_norm = re.sub(r'[^\w\s]', '', h).strip().lower()
        if normalized == h_norm:
            return True
    return False

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
    
    if _is_hallucination(transcription):
        logger.info(f"Filtered STT hallucination: '{transcription.strip()}'")
        return ""
        
    return transcription
