import asyncio
import json
import re
import uuid
from typing import List, Dict
from fastapi import WebSocket, WebSocketDisconnect

from app.stt_service import transcribe_audio
from app.llm_service import generate_response_stream
from app.elevenlabs_service import generate_audio_stream
from app.s3_service import archive_session_data

# A regular expression used for sentence boundary detection
# We split on punctuation marks that strongly denote end-of-sentence in German.
SENTENCE_BOUNDARY_REGEX = re.compile(r'([.?!])\s*')

class ConnectionManager:
    def __init__(self):
        self.active_connections: list[WebSocket] = []

    async def connect(self, websocket: WebSocket):
        await websocket.accept()
        self.active_connections.append(websocket)

    def disconnect(self, websocket: WebSocket):
        if websocket in self.active_connections:
            self.active_connections.remove(websocket)

manager = ConnectionManager()

async def websocket_endpoint(websocket: WebSocket):
    await manager.connect(websocket)
    session_id = str(uuid.uuid4())
    
    # Store conversation history
    messages: List[Dict[str, str]] = []

    try:
        # Phase 1: Initialization (Client sends system prompt and instructions first)
        init_data_str = await websocket.receive_text()
        try:
            init_data = json.loads(init_data_str)
            system_prompt = init_data.get("system_prompt", "You are a helpful German examiner.")
            user_instructions = init_data.get("user_instructions", "")
            
            # Formulate the initial system context
            full_system_prompt = f"{system_prompt}\nConstraints: {user_instructions}"
            messages.append({"role": "system", "content": full_system_prompt})
            
            # Let the client know Phase 1 is done
            await websocket.send_text(json.dumps({"status": "initialized", "session_id": session_id}))
        except json.JSONDecodeError:
            await websocket.send_text(json.dumps({"error": "Phase 1 requires a JSON object with system_prompt and user_instructions"}))
            await websocket.close()
            return

        # Phase 2: The Loop (Wait for binary audio frames)
        while True:
            # Receive binary audio chunk from the frontend
            data = await websocket.receive_bytes()
            
            # --- STT Phase ---
            # Using Whisper STT 
            transcribed_text = await transcribe_audio(data)
            
            if not transcribed_text.strip():
                continue
                
            messages.append({"role": "user", "content": transcribed_text})
            
            # Signal the client that response is starting, and supply the transcription text
            await websocket.send_text(json.dumps({
                "status": "generating_response", 
                "transcribed": transcribed_text
            }))

            # --- LLM + TTS Streaming Pipelining (Glass Pipe) ---
            sentence_buffer = ""
            full_llm_response = ""
            all_generated_audio_bytes = bytearray()
            
            # Generator for LLM tokens
            async for token in generate_response_stream(messages):
                sentence_buffer += token
                full_llm_response += token
                
                # Check for sentence boundary for perfectly prosodic German TTS
                if bool(SENTENCE_BOUNDARY_REGEX.search(sentence_buffer)):
                    splits = SENTENCE_BOUNDARY_REGEX.split(sentence_buffer, 1)
                    if len(splits) == 3:
                        sentence = splits[0] + splits[1] # Sentence + Punctuation
                        remainder = splits[2]
                        
                        if sentence.strip():
                            # Send this full sentence to TTS immediately!
                            # Stream audio chunks directly back to the WebSocket
                            async for audio_chunk in generate_audio_stream(sentence):
                                await websocket.send_bytes(audio_chunk)
                                all_generated_audio_bytes.extend(audio_chunk)
                                
                        sentence_buffer = remainder
            
            # Process any remainder in the sentence buffer after LLM finishes
            if sentence_buffer.strip():
                async for audio_chunk in generate_audio_stream(sentence_buffer):
                    await websocket.send_bytes(audio_chunk)
                    all_generated_audio_bytes.extend(audio_chunk)
                    
            messages.append({"role": "assistant", "content": full_llm_response})
            
            # Signal end of transmission for this turn (text included so client can show subtitles)
            await websocket.send_text(json.dumps({
                "status": "response_complete",
                "ai_text": full_llm_response
            }))

            # --- Archiving Phase ---
            metadata = {
                "transcription": transcribed_text,
                "llm_response": full_llm_response,
            }
            # Fire off S3 upload async without awaiting
            asyncio.create_task(archive_session_data(session_id, metadata, bytes(all_generated_audio_bytes)))

    except WebSocketDisconnect:
        manager.disconnect(websocket)
    except Exception as e:
        manager.disconnect(websocket)
        print(f"WebSocket Error: {e}")
