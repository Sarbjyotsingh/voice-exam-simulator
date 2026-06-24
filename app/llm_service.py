import logging
import json
from groq import AsyncGroq
from typing import List, Dict
from app.config import settings

logger = logging.getLogger("app.llm")
client = AsyncGroq(api_key=settings.groq_api_key)

async def get_structured_response(messages: List[Dict[str, str]]) -> str:
    """
    Calls Groq's Llama 3.3 model in JSON mode and returns the full string response.
    """
    logger.info("Requesting Llama 3.3 completion (JSON Mode)...")
    completion = await client.chat.completions.create(
        messages=messages,
        model="llama-3.3-70b-versatile",
        response_format={"type": "json_object"},
        stream=False,
    )
    return completion.choices[0].message.content or "{}"

async def get_evaluation_response(prompt: str, transcript: str) -> str:
    """
    Calls Groq's Llama 3.3 model to generate the evaluation report in JSON format.
    """
    json_instruction = (
        "\n\nYou MUST return your response as a valid JSON object matching the following structure:\n"
        "{\n"
        '  "numerical_score": "The numerical score (e.g., 20 / 25)",\n'
        '  "telc_grade": "The Telc grade (e.g., Pass, Pass with Distinction, Not yet B1)",\n'
        '  "top_3_mistakes": ["Mistake 1", "Mistake 2", "Mistake 3"],\n'
        '  "interaction_feedback": "Detailed feedback on interaction",\n'
        '  "foreigner_success_tip": "One specific success tip"\n'
        "}\n"
        "Do not include any other text, markdown formatting, or explanations outside the JSON."
    )
    full_prompt = f"{prompt}\n{json_instruction}"

    messages = [
        {"role": "system", "content": full_prompt},
        {"role": "user", "content": transcript}
    ]
    logger.info("Requesting Llama 3.3 completion for Evaluation (JSON Schema)...")
    completion = await client.chat.completions.create(
        messages=messages,
        model="llama-3.3-70b-versatile",
        response_format={"type": "json_object"},
        stream=False,
    )
    content = completion.choices[0].message.content or "{}"

    try:
        data = json.loads(content)
        # Verify keys are present
        required_keys = ["numerical_score", "telc_grade", "top_3_mistakes", "interaction_feedback", "foreigner_success_tip"]
        for key in required_keys:
            if key not in data:
                data[key] = "" if key != "top_3_mistakes" else []
        return json.dumps(data)
    except Exception as e:
        logger.error(f"Failed to parse or validate LLM JSON response: {e}. Raw content: {content}")
        return content

