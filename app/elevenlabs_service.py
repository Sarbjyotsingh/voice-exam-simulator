import httpx
from typing import AsyncGenerator
from app.config import settings

async def generate_audio_stream(text: str) -> AsyncGenerator[bytes, None]:
    """
    Calls ElevenLabs streaming API to get TTS audio chunk by chunk.
    Utilizes HTTPX async streams to optimize time to first byte.
    """
    url = f"https://api.elevenlabs.io/v1/text-to-speech/{settings.elevenlabs_voice_id}/stream"
    headers = {
        "Accept": "audio/mpeg",
        "Content-Type": "application/json",
        "xi-api-key": settings.elevenlabs_api_key
    }
    data = {
        "text": text,
        "model_id": "eleven_flash_v2_5",
        "voice_settings": {
            "stability": 0.5,
            "similarity_boost": 0.5
        }
    }

    async with httpx.AsyncClient() as client:
        # We use a POST stream for immediate TTFB
        async with client.stream("POST", url, json=data, headers=headers) as response:
            response.raise_for_status() # Raise error if bad request (e.g. invalid auth)
            async for chunk in response.aiter_bytes(chunk_size=4096):
                yield chunk
