import httpx
import logging
import json
from typing import AsyncGenerator
from app.config import settings

logger = logging.getLogger("app.tts")

async def generate_audio_stream(text: str, voice_id: str = None) -> AsyncGenerator[bytes, None]:
    """
    Calls ElevenLabs streaming API to get TTS audio chunk by chunk.
    """
    active_voice_id = voice_id or settings.elevenlabs_voice_id
    voice_source = "passed/mapping" if voice_id else "global_default"
    
    url = f"https://api.elevenlabs.io/v1/text-to-speech/{active_voice_id}/stream"
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

    logger.info(f"🎤 ElevenLabs TTS (Streaming) | Voice: {active_voice_id} | Text: '{text}'")

    async with httpx.AsyncClient() as client:
        try:
            async with client.stream("POST", url, json=data, headers=headers) as response:
                if response.status_code != 200:
                    error_text = await response.aread()
                    logger.error(f"ElevenLabs TTS failed: Status {response.status_code} - {error_text.decode()}")
                    response.raise_for_status()
                
                logger.info("ElevenLabs stream established. Receiving chunks...")
                async for chunk in response.aiter_bytes(chunk_size=4096):
                    yield chunk
        except Exception as e:
            logger.error(f"Error during TTS streaming: {str(e)}")
            raise e

async def generate_audio_final(text: str, voice_id: str = None) -> bytes:
    """
    Calls ElevenLabs API to get the entire TTS audio product at once.
    """
    active_voice_id = voice_id or settings.elevenlabs_voice_id
    voice_source = "passed/mapping" if voice_id else "global_default"
    
    url = f"https://api.elevenlabs.io/v1/text-to-speech/{active_voice_id}"
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

    logger.info(f"🎤 ElevenLabs TTS (Final) | Voice: {active_voice_id} | Text: '{text}'")

    async with httpx.AsyncClient(timeout=60.0) as client:
        try:
            response = await client.post(url, json=data, headers=headers)
            if response.status_code != 200:
                logger.error(f"ElevenLabs TTS failed: Status {response.status_code} - {response.text}")
                response.raise_for_status()
            
            logger.info("ElevenLabs audio generated successfully.")
            return response.content
        except Exception as e:
            logger.error(f"Error during TTS generation: {str(e)}")
            raise e
