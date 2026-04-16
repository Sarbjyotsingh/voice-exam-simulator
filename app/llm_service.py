from groq import AsyncGroq
from typing import AsyncGenerator, List, Dict
from app.config import settings

client = AsyncGroq(api_key=settings.groq_api_key)

async def generate_response_stream(messages: List[Dict[str, str]]) -> AsyncGenerator[str, None]:
    """
    Streams tokens from Groq's Llama 3.3 model given the message history.
    Yields them chunk by chunk for the Glass Pipe architecture.
    """
    stream = await client.chat.completions.create(
        messages=messages,
        model="llama-3.3-70b-versatile",
        stream=True,
    )
    
    async for chunk in stream:
        content = chunk.choices[0].delta.content
        if content:
            yield content
