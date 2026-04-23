import httpx
import logging
from app.config import settings

logger = logging.getLogger("app.tts")

async def generate_audio_final(text: str, voice_id: str = None) -> bytes:
    """
    Calls ElevenLabs API to get the entire TTS audio product at once.
    """
    active_voice_id = voice_id or settings.elevenlabs_voice_id
    
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

    logger.info(f"🎤 ElevenLabs TTS | Voice: {active_voice_id} | Text: '{text[:50]}...'")

    async with httpx.AsyncClient(timeout=60.0) as client:
        try:
            response = await client.post(url, json=data, headers=headers)
            if response.status_code != 200:
                logger.error(f"ElevenLabs TTS failed: {response.status_code} - {response.text}")
                response.raise_for_status()
            
            return response.content
        except Exception as e:
            logger.error(f"Error during TTS generation: {e}")
            raise e
