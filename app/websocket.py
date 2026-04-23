import asyncio
import json
import uuid
import logging
from typing import List, Dict
from fastapi import WebSocket, WebSocketDisconnect

from app.stt_service import transcribe_audio
from app.llm_service import get_structured_response
from app.elevenlabs_service import generate_audio_final
from app.s3_service import archive_session_data

logger = logging.getLogger("app.websocket")

class ConnectionManager:
    def __init__(self):
        self.active_connections: list[WebSocket] = []

    async def connect(self, websocket: WebSocket):
        await websocket.accept()
        self.active_connections.append(websocket)
        logger.info(f"WebSocket Connected. Active: {len(self.active_connections)}")

    def disconnect(self, websocket: WebSocket):
        if websocket in self.active_connections:
            self.active_connections.remove(websocket)
        logger.info(f"WebSocket Disconnected. Active: {len(self.active_connections)}")

manager = ConnectionManager()

async def websocket_endpoint(websocket: WebSocket):
    await manager.connect(websocket)
    session_id = str(uuid.uuid4())
    messages: List[Dict[str, str]] = []

    try:
        # Phase 1: Initialization
        init_data_str = await websocket.receive_text()
        try:
            init_data = json.loads(init_data_str)
            system_prompt = init_data.get("system_prompt", "You are a helpful German examiner.")
            user_instructions = init_data.get("user_instructions", "")
            history = init_data.get("history", [])
            voice_mapping = init_data.get("voice_mapping", {})
            normalized_mapping = {k.lower(): v for k, v in voice_mapping.items()}
            
            session_default_voice_id = init_data.get("voice_id")
            if not session_default_voice_id:
                for key in ["default", "examiner", "assistant"]:
                    if key in normalized_mapping:
                        session_default_voice_id = normalized_mapping[key]
                        break
            
            json_instruction = "\n\nYou MUST return your response as a valid JSON object containing an array called 'parts'. Each item in the array must be an object with optional keys: 'text', 'audio_id', 'speaker', and 'role'. Do not include any other markdown formatting."
            full_system_prompt = f"{system_prompt}\nConstraints: {user_instructions}{json_instruction}"
            messages.append({"role": "system", "content": full_system_prompt})
            
            if history:
                messages.extend([m for m in history if m.get("role") != "system"])
            
            logger.info(f"Session {session_id} initialized. Default Voice: {session_default_voice_id}")
            await websocket.send_text(json.dumps({"status": "initialized", "session_id": session_id}))
        except json.JSONDecodeError:
            logger.error("Initialization failed: Invalid JSON")
            await websocket.send_text(json.dumps({"error": "Invalid initialization JSON"}))
            await websocket.close()
            return

        # Phase 2: Interaction Loop
        while True:
            data = await websocket.receive_bytes()
            logger.info(f"Received {len(data)} bytes audio.")
            
            transcribed_text = await transcribe_audio(data)
            if not transcribed_text.strip():
                continue
                
            messages.append({"role": "user", "content": transcribed_text})
            await websocket.send_text(json.dumps({
                "status": "generating_response", 
                "transcribed": transcribed_text
            }))

            current_voice_id = session_default_voice_id 
            full_llm_response = await get_structured_response(messages)
            all_generated_audio_bytes = bytearray()

            try:
                response_data = json.loads(full_llm_response)
                parts = response_data.get("parts", [])
            except json.JSONDecodeError:
                logger.error("Failed to parse LLM JSON")
                parts = []
            
            for part in parts:
                audio_id = part.get("audio_id") or part.get("audioId")
                text = part.get("text")
                speaker = part.get("speaker") or part.get("role")
                
                if speaker:
                    speaker_key = speaker.lower()
                    if speaker_key in normalized_mapping:
                        current_voice_id = normalized_mapping[speaker_key]
                
                if audio_id:
                    await websocket.send_text(json.dumps({
                        "status": "play_audio_id",
                        "audio_id": audio_id
                    }))
                elif text:
                    clean_text = text.strip()
                    if clean_text:
                        audio_bytes = await generate_audio_final(clean_text, voice_id=current_voice_id)
                        await websocket.send_bytes(audio_bytes)
                        all_generated_audio_bytes.extend(audio_bytes)

            messages.append({"role": "assistant", "content": full_llm_response})
            
            await websocket.send_text(json.dumps({
                "status": "response_complete",
                "ai_text": full_llm_response,
                "parts": parts
            }))

            asyncio.create_task(archive_session_data(
                session_id, 
                {"transcription": transcribed_text, "llm_response": full_llm_response}, 
                bytes(all_generated_audio_bytes)
            ))

    except WebSocketDisconnect:
        manager.disconnect(websocket)
    except Exception as e:
        logger.exception(f"Error in WebSocket loop: {e}")
        manager.disconnect(websocket)
