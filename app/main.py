from fastapi import FastAPI, WebSocket
from app.websocket import websocket_endpoint

app = FastAPI(title="telc German Exam Simulator API")

@app.get("/health")
async def health_check():
    """
    Health check endpoint for AWS Application Load Balancer.
    Returns 200 OK to indicate the container is healthy.
    """
    return {"status": "healthy"}

@app.websocket("/exam")
async def exam_websocket(websocket: WebSocket):
    """
    Main WebSocket endpoint routing to the connection manager logic.
    """
    await websocket_endpoint(websocket)
