import logging
from groq import AsyncGroq
from typing import AsyncGenerator, List, Dict
from app.config import settings

logger = logging.getLogger("app.llm")
client = AsyncGroq(api_key=settings.groq_api_key)

async def get_structured_response(messages: List[Dict[str, str]]) -> str:
    """
    Calls Groq's Llama 3.3 model in JSON mode and returns the full string response.
    """
    # VERBOSE DEBUG: Print exactly what is being sent to the AI
    logger.info("================ LLM DEBUG START ================")
    logger.info(f"Model: llama-3.3-70b-versatile")
    logger.info("Messages:")
    for idx, msg in enumerate(messages):
        logger.info(f"  {idx} [{msg['role'].upper()}]: {msg['content']}")
    logger.info("================= LLM DEBUG END =================")
    
    logger.info("Requesting Llama 3.3 completion (JSON Mode)...")
    completion = await client.chat.completions.create(
        messages=messages,
        model="llama-3.3-70b-versatile",
        response_format={"type": "json_object"},
        stream=False,
    )
    
    content = completion.choices[0].message.content
    return content or "{}"
