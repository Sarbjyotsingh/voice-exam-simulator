package config

import "time"

const (
	// Groq endpoints and model parameters
	GroqBaseURL      = "https://api.groq.com/openai/v1"
	GroqWhisperModel = "whisper-large-v3"
	GroqSTTLanguage  = "de"

	// ElevenLabs endpoints and model parameters
	ElevenLabsBaseURL = "https://api.elevenlabs.io/v1"
	ElevenLabsModel   = "eleven_flash_v2_5"

	// Default unit prices (USD). Overridable via env, see config.MustLoad.
	DefaultWhisperPricePerHourUSD       = 0.111 // Groq whisper-large-v3, per audio hour
	DefaultGroqInputPer1MUSD            = 0.15  // Groq chat model input, per 1M tokens
	DefaultGroqOutputPer1MUSD           = 0.60  // Groq chat model output, per 1M tokens
	DefaultElevenLabsFlashPer1KCharsUSD = 0.05  // eleven_flash_v2_5 = 0.5 credit/char

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
	DefaultSystemPrompt = "You are a helpful German language speaking partner."

	// LLM JSON Instruction
	JSONInstruction = "\n\nYou MUST return your response as a valid JSON object containing an array called 'parts' and an optional boolean key 'exam_ended'. Each item in the array must be an object with optional keys: 'text', 'audio_id', 'speaker', and 'role'. Do not include any other markdown formatting."

	EvaluationJSONInstruction = "\n\nYou MUST return your response as a valid JSON object matching the following structure:\n{\n  \"numerical_score\": \"The numerical score (e.g., 20 / 25)\",\n  \"telc_grade\": \"The Telc grade (e.g., Pass, Pass with Distinction, Not yet B1)\",\n  \"top_3_mistakes\": [\"Mistake 1\", \"Mistake 2\", \"Mistake 3\"],\n  \"interaction_feedback\": \"Detailed feedback on interaction\",\n  \"foreigner_success_tip\": \"One specific success tip\"\n}\nDo not include any other text, markdown formatting, or explanations outside the JSON."
)

// VoiceCostMultipliers maps an ElevenLabs voice ID to its credit multiplier.
// Standard voices are 1.0; community/custom-rate voices can be higher. Unknown voices default to 1.0.
var VoiceCostMultipliers = map[string]float64{
	// Examiners
	"Jvf6TAXwMUVTSR20U0f9": 1.0, // Herr Wagner
	"MGG5Irb57ATHvyIeTEYo": 1.0, // Frau Fischer
	"zKHQdbB8oaQ7roNTiDTK": 1.0, // Frau Erdoğan
	"z1EhmmPwF0ENGYE8dBE6": 1.0, // Herr Richter
	"4YKI1ONIhgWEKHyAHYPC": 1.0, // Frau Dr. Meyer
	// Partner personas
	"NE7AIW5DoJ7lUosXV2KR": 1.0, // Leyla
	"A2eYGaZmwykOiEVBlQ7h": 1.0, // Omar
	"gVOibprogMfmHVVyo5r6": 1.0, // Elena
	"1J0wWp4zPQIvsK7Xwh34": 1.0, // Mateo
	"K5ZVtkkBnuPY6YqXs70E": 1.0, // Janusz
}
