import asyncio
import json
import re
import uuid
import logging
from typing import List, Dict
from fastapi import WebSocket, WebSocketDisconnect

from app.stt_service import transcribe_audio
from app.llm_service import get_structured_response
from app.elevenlabs_service import generate_audio_stream, generate_audio_final
from app.s3_service import archive_session_data

logger = logging.getLogger("app.websocket")

# Regex constants removed as we now use Structured Outputs (JSON)

class ConnectionManager:
    def __init__(self):
        self.active_connections: list[WebSocket] = []

    async def connect(self, websocket: WebSocket):
        await websocket.accept()
        self.active_connections.append(websocket)
        logger.info(f"WebSocket Client Connected. Active connections: {len(self.active_connections)}")

    def disconnect(self, websocket: WebSocket):
        if websocket in self.active_connections:
            self.active_connections.remove(websocket)
        logger.info(f"WebSocket Client Disconnected. Active connections: {len(self.active_connections)}")

manager = ConnectionManager()

async def websocket_endpoint(websocket: WebSocket):
    await manager.connect(websocket)
    session_id = str(uuid.uuid4())
    logger.info(f"New Session ID generated: {session_id}")
    
    # Store conversation history
    messages: List[Dict[str, str]] = []

    try:
        # Phase 1: Initialization (Client sends system prompt and instructions first)
        logger.info("Waiting for Phase 1: Session Initialization...")
        init_data_str = await websocket.receive_text()
        try:
            init_data = json.loads(init_data_str)
            system_prompt = init_data.get("system_prompt", "You are a helpful German examiner.")
            user_instructions = init_data.get("user_instructions", "")
            history = init_data.get("history", [])
            voice_mapping = init_data.get("voice_mapping", {})
            
            # 1. Normalize mapping to lowercase for robust lookup
            normalized_mapping = {k.lower(): v for k, v in voice_mapping.items()}
            
            # 2. Determine a Session-Level Default Voice ID
            # Priority: explicit 'voice_id' > 'default' in mapping > 'examiner' > 'assistant' > first entry if unique
            session_default_voice_id = init_data.get("voice_id")
            if not session_default_voice_id:
                for key in ["default", "examiner", "assistant"]:
                    if key in normalized_mapping:
                        session_default_voice_id = normalized_mapping[key]
                        break
            
            # Strict constraint: No default voice fallback. LLM must explicitly map to a speaker.
            # If session_default_voice_id is None, TTS will fall back to the base model default.
            if not session_default_voice_id:
                logger.info("No session default voice ID matched. Relying on per-sentence tags.")

            logger.info(f"Session Default Voice ID: {session_default_voice_id}")

            # Always formulate the system context and ensure it is the first message
            json_instruction = "\n\nYou MUST return your response as a valid JSON object containing an array called 'parts'. Each item in the array must be an object with optional keys: 'text', 'audio_id', 'speaker', and 'role'. Do not include any other markdown formatting."
            full_system_prompt = f"{system_prompt}\nConstraints: {user_instructions}{json_instruction}"
            messages.append({"role": "system", "content": full_system_prompt})
            
            if history:
                # Filter out any 'system' messages from history to avoid duplicates
                filtered_history = [m for m in history if m.get("role") != "system"]
                messages.extend(filtered_history)
            
            logger.info(f"🚀 Session Init | Default Voice ID: {session_default_voice_id} | History Length: {len(history)}")
            
            logger.info(f"Session Initialized. ID: {session_id}")
            # Let the client know Phase 1 is done
            await websocket.send_text(json.dumps({"status": "initialized", "session_id": session_id}))
        except json.JSONDecodeError:
            logger.error("Phase 1 failed: Client did not send valid JSON.")
            await websocket.send_text(json.dumps({"error": "Phase 1 requires a JSON object with system_prompt and user_instructions"}))
            await websocket.close()
            return

        # Phase 2: The Loop (Wait for binary audio frames)
        while True:
            logger.info("Waiting for binary audio input from client...")
            # Receive binary audio chunk from the frontend
            data = await websocket.receive_bytes()
            logger.info(f"Received {len(data)} bytes of audio data.")
            
            # --- STT Phase ---
            logger.info("[DATA FLOW] Client Binary --(Audio Bytes)--> STT Service")
            try:
                transcribed_text = await transcribe_audio(data)
                logger.info(f"[DATA FLOW] STT Output: '{transcribed_text}'")
            except Exception as e:
                logger.error(f"STT Failed: {str(e)}")
                raise e
            
            if not transcribed_text.strip():
                logger.warning("Empty transcription received. Skipping this turn.")
                continue
                
            messages.append({"role": "user", "content": transcribed_text})
            
            # Signal the client that response is starting, and supply the transcription text
            await websocket.send_text(json.dumps({
                "status": "generating_response", 
                "transcribed": transcribed_text
            }))

            # --- LLM + TTS Final Product (No Streaming) ---
            full_llm_response = ""
            all_generated_audio_bytes = bytearray()
            
            # TRACK CURRENT VOICE ID (Initialize with session default)
            current_voice_id = session_default_voice_id 

            logger.info("[DATA FLOW] Requesting full structured LLM response...")
            full_llm_response = await get_structured_response(messages)
                
            logger.info(f"Full LLM response generated: '{full_llm_response}'")

            # Process the full structured JSON response
            try:
                response_data = json.loads(full_llm_response)
                parts = response_data.get("parts", [])
            except json.JSONDecodeError as e:
                logger.error(f"Failed to parse LLM JSON: {e}")
                parts = []
            
            for part in parts:
                audio_id = part.get("audio_id") or part.get("audioId")
                text = part.get("text")
                speaker = part.get("speaker") or part.get("role")
                
                # 1. Resolve speaker to voice ID if provided
                if speaker:
                    speaker_key = speaker.lower()
                    if speaker_key in normalized_mapping:
                        current_voice_id = normalized_mapping[speaker_key]
                        logger.info(f"[VOICE SWITCH] Role '{speaker}' detected. Using Voice ID: {current_voice_id}")
                    else:
                        logger.warning(f"Speaker '{speaker}' not found in voice_mapping. Falling back to {current_voice_id}")
                
                # 2. Handle Pre-recorded Audio Signals
                if audio_id:
                    logger.info(f"[SIGNAL] Audio ID detected in response: {audio_id}. Notifying client...")
                    await websocket.send_text(json.dumps({
                        "status": "play_audio_id",
                        "audio_id": audio_id
                    }))
                
                # 3. Handle Text-to-Speech Generation (Only if no pre-recorded audio_id was provided)
                elif text:
                    clean_text = text.strip()
                    if clean_text:
                        logger.info(f"[DATA FLOW] Final Product Request: '{clean_text}' --(Text, Voice: {current_voice_id})--> TTS Service")
                        audio_bytes = await generate_audio_final(clean_text, voice_id=current_voice_id)
                        await websocket.send_bytes(audio_bytes)
                        all_generated_audio_bytes.extend(audio_bytes)

            logger.info("[DATA FLOW] Final TTS buffer complete.")
                    
            messages.append({"role": "assistant", "content": full_llm_response})
            logger.info(f"Full LLM response: '{full_llm_response}'")
            
            # Signal end of transmission for this turn
            # Send the parsed parts directly so the Flutter app can deserialize into LLMResponsePart list
            await websocket.send_text(json.dumps({
                "status": "response_complete",
                "ai_text": full_llm_response,
                "parts": parts
            }))

            # --- Archiving Phase ---
            metadata = {
                "transcription": transcribed_text,
                "llm_response": full_llm_response,
            }
            # Fire off S3 upload async without awaiting
            logger.info("Dispatching background archival to S3...")
            asyncio.create_task(archive_session_data(session_id, metadata, bytes(all_generated_audio_bytes)))

    except WebSocketDisconnect:
        manager.disconnect(websocket)
    except Exception as e:
        logger.exception(f"Fatal exception in WebSocket loop: {str(e)}")
        manager.disconnect(websocket)
