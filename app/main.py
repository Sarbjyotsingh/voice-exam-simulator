import logging
from fastapi import FastAPI, WebSocket
from app.websocket import websocket_endpoint

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s"
)
logger = logging.getLogger("app.main")

app = FastAPI(title="telc German Exam Simulator API")

@app.on_event("startup")
async def startup_event():
    logger.info("API Started")

@app.get("/health")
async def health_check():
    return {"status": "healthy"}

@app.websocket("/exam")
async def exam_websocket(websocket: WebSocket):
    await websocket_endpoint(websocket)
