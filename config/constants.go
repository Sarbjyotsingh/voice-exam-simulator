package config

import "time"

const (
	// Groq endpoints and model parameters
	GroqBaseURL        = "https://api.groq.com/openai/v1"
	GroqWhisperModel   = "whisper-large-v3"
	GroqSTTLanguage    = "de"

	// ElevenLabs endpoints and model parameters
	ElevenLabsBaseURL = "https://api.elevenlabs.io/v1"
	ElevenLabsModel   = "eleven_flash_v2_5"

	// WebSocket Status Codes
	StatusInitialized        = "initialized"
	StatusGeneratingResponse = "generating_response"
	StatusEmptyTranscription = "empty_transcription"
	StatusPlayAudioID        = "play_audio_id"
	StatusResponseComplete   = "response_complete"
	StatusError              = "error"

	// Ping period
	PingPeriod = 30 * time.Second
	PongWait   = 90 * time.Second

	// Default System Prompts
	DefaultSystemPrompt = "You are a helpful German examiner."

	// LLM JSON Instruction
	JSONInstruction = "\n\nYou MUST return your response as a valid JSON object containing an array called 'parts' and an optional boolean key 'exam_ended'. Each item in the array must be an object with optional keys: 'text', 'audio_id', 'speaker', and 'role'. Do not include any other markdown formatting."

	EvaluationJSONInstruction = "\n\nYou MUST return your response as a valid JSON object matching the following structure:\n{\n  \"numerical_score\": \"The numerical score (e.g., 20 / 25)\",\n  \"telc_grade\": \"The Telc grade (e.g., Pass, Pass with Distinction, Not yet B1)\",\n  \"top_3_mistakes\": [\"Mistake 1\", \"Mistake 2\", \"Mistake 3\"],\n  \"interaction_feedback\": \"Detailed feedback on interaction\",\n  \"foreigner_success_tip\": \"One specific success tip\"\n}\nDo not include any other text, markdown formatting, or explanations outside the JSON."
)
