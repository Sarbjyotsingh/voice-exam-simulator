import logging
from fastapi import FastAPI, WebSocket
from pydantic import BaseModel
from app.websocket import websocket_endpoint
from app.llm_service import get_evaluation_response

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s"
)
logger = logging.getLogger("app.main")

app = FastAPI(title="telc German Exam Simulator API")

class EvaluationRequest(BaseModel):
    evaluation_prompt: str
    transcript: str

@app.on_event("startup")
async def startup_event():
    logger.info("API Started")

@app.get("/health")
async def health_check():
    return {"status": "healthy"}

@app.post("/evaluate")
async def evaluate_exam(request: EvaluationRequest):
    logger.info("Received evaluation request")
    result = await get_evaluation_response(request.evaluation_prompt, request.transcript)
    return {"result": result}

@app.websocket("/exam")
async def exam_websocket(websocket: WebSocket):
    await websocket_endpoint(websocket)

