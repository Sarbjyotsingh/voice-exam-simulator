import httpx
import asyncio
import os
from dotenv import load_dotenv

load_dotenv()

async def list_voices():
    api_key = os.getenv("ELEVENLABS_API_KEY")
    if not api_key:
        print("Error: ELEVENLABS_API_KEY not found in environment.")
        return

    url = "https://api.elevenlabs.io/v1/voices"
    headers = {"xi-api-key": api_key}

    async with httpx.AsyncClient() as client:
        response = await client.get(url, headers=headers)
        if response.status_code == 200:
            voices = response.json().get("voices", [])
            print(f"Found {len(voices)} voices:")
            for voice in voices:
                print(f"- {voice['name']} (ID: {voice['voice_id']})")
        else:
            print(f"Error fetching voices: {response.status_code} - {response.text}")

if __name__ == "__main__":
    asyncio.run(list_voices())
