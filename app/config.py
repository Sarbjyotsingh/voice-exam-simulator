from pydantic_settings import BaseSettings

class Settings(BaseSettings):
    groq_api_key: str
    elevenlabs_api_key: str
    elevenlabs_voice_id: str = "pNInz6obbfDQGcgMyIGD"  # Example voice for telc examiner
    aws_region: str = "eu-central-1"
    s3_bucket_name: str = "telc-exam-simulator-sessions"
    
    class Config:
        env_file = ".env"

settings = Settings()
