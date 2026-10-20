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
	DefaultDecisionsInputPer1MUSD       = 0.10  // OpenAI Decisions API (gpt-6-luna), per 1M input tokens

	// OpenAI endpoints and model parameters
	OpenAIBaseURL        = "https://api.openai.com/v1"
	OpenAIDecisionsModel = "gpt-6-luna"

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

	// Groq Whisper initial_prompt to bias STT towards Telc B1 speaking vocabulary and reduce phonetic hallucinations
	GroqWhisperPrompt = "Mündliche Prüfung telc Deutsch B1, Teil 1 Kontaktaufnahme. Hallo, wollen wir du oder Sie sagen? Ich heiße, mein Vorname, mein Nachname, Familienname, buchstabieren. Ich komme aus, ich wohne in, Wohnung, Wohngemeinschaft, WG, verheiratet, ledig, Familie, Kinder. Ich arbeite als Softwareentwickler, Ausbildung, Studium, Beruf, Muttersprache, Integrationskurs, Sprachschule, Volkshochschule, Freizeit, Hobbys, spazieren gehen. Und du? Woher kommst du? Wo wohnst du? Was machst du beruflich?"

	// Default System Prompts
	DefaultSystemPrompt = "You are a helpful German language speaking partner."

	// LLM JSON Instruction
	JSONInstruction = "\n\nYou MUST return your response as a valid JSON object containing an array called 'parts' and an optional boolean key 'exam_ended'. Each item in the array must be an object with optional keys: 'text', 'audio_id', 'speaker', and 'role'. Do not include any other markdown formatting."

	// EvaluationGuardrails enforces STT-aware error reconstruction, fair Telc B1 grading, and high-quality corrections on the backend.
	EvaluationGuardrails = `

### CRITICAL STT & EVALUATION GUARDRAILS (HIGHEST PRIORITY — OVERRIDES ALL OTHER INSTRUCTIONS):
1. **LANGUAGE REQUIREMENT:** Write ALL evaluation fields ("interaction_feedback", "foreigner_success_tip", and any explanatory notes) in **clear, encouraging English**, while keeping German example phrases and sentence corrections in **German**.
2. **SPEECH-TO-TEXT (STT) PHONETIC RECONSTRUCTION (CRITICAL):**
   - The Candidate's lines were transcribed automatically by Whisper Speech-to-Text (STT) from a non-native speaker's audio.
   - Whisper frequently produces **phonetic hallucinations** (garbled German words that sound similar to what the learner actually said). Common examples:
     * "und willkommen zu" or "woher kommst zu" at the end of a turn -> The learner actually said **"Und woher kommst du?"** (or "Und wie heißt du?").
     * "eine Software in Quickler arbeiten" -> The learner actually said **"als Softwareentwickler arbeiten"** ("Entwickler" misheard as "in Quickler").
     * "Mein Name ist Mutterschweiger" -> The learner was answering "Was ist deine Muttersprache?" ("Meine Muttersprache ist...").
     * "Sarab" / "Saurabh" -> Learner's name.
   - **YOUR MANDATE:** Always infer the **intended spoken German phrase** from phonetic similarity and conversational context (especially the partner's preceding question and subsequent reaction/answer):
     * **NEVER** treat an STT phonetic hallucination literally (e.g., NEVER tell the learner to say "bei Quickler" or "ich bin hier, um Sie kennenzulernen" when they meant "als Softwareentwickler" or "Und woher kommst du?").
     * If the learner's intended phrase was grammatically correct (e.g., "Und woher kommst du?" misheard as "und willkommen zu"), **DO NOT list it as a mistake** and **give full credit** for asking the question!
     * If the learner made a real spoken grammar error alongside an STT artifact (e.g., "Ich will hier eine Software in Quickler arbeiten" where they used "eine" instead of "als" and repeated words), reconstruct the **true intended target sentence** in your correction: Quote what they tried to say and provide the natural B1 correction (e.g., *"\"Ich will hier als Softwareentwickler arbeiten.\" → Better: \"Ich möchte hier als Softwareentwickler arbeiten.\""*).
3. **NEVER PENALIZE TYPING / PUNCTUATION / CAPITALIZATION:**
   - A human cannot speak capital letters or commas!
   - **STRICTLY FORBIDDEN in top_3_mistakes:** NEVER criticize lowercase words (e.g., "hindi, englisch und deutsch"), comma splices, missing periods, or hyphenation.
4. **WHAT BELONGS IN "top_3_mistakes":**
   - Only include **actual spoken German grammar, vocabulary, or syntax mistakes** made by the ` + "`Candidate`" + ` (e.g., wrong verb placement, wrong preposition, missing article/preposition like "arbeiten" without "als", wrong verb-noun collocation like "Spaziergang gehen" instead of "spazieren gehen", broken sentence structure like "Ich habe meine Frau wohnt mit mir", or wrong gender like "Meine Familienname" instead of "Mein Familienname").
   - Format each item strictly as: ` + "`\"[Spoken error] → [Natural B1 German correction] ([Short 3-6 word reason in English])\"`" + `.
   - Do NOT put "missing topics" or "did not answer X" in ` + "`top_3_mistakes`" + ` unless the candidate made fewer than 3 spoken language errors. Discuss topic coverage and turn-taking in ` + "`interaction_feedback`" + `.
5. **ACCURATE TELC B1 CONTEXT & SCORING RUBRIC:**
   - **Du/Sie Rule:** Check the partner's opening line (` + "`OPEN_01`" + `). Since the partner already proposed *"du"* and immediately asked *"Und wie heißt du?"*, answering directly in the *"du"* form (e.g., *"Woher kommst du?"*) is **100% natural and implicitly accepts "du"**. NEVER penalize the candidate for not explicitly saying *"Ja, wir können du sagen"* if they used *"du"* naturally!
   - **Pre-recorded Partner Flow:** In this simulator, the AI partner drives the topic transitions and concludes after ~6–7 turns. Do NOT penalize the candidate if the AI partner skipped asking one of the sub-topics (e.g. "Wo Deutsch gelernt" or "Hobbys") before closing. Evaluate the candidate on how well they answered the questions that *were* asked and whether they actively asked counter-questions back.
   - **Calibrated Scoring (0–25 points across 5 criteria x 5 pts):**
     * **22–25 pts ("Pass with Distinction"):** Complete dialogue, natural back-and-forth questions, clear B1 sentences with minor errors.
     * **15–21 pts ("Pass"):** Communicative goal achieved! Even with noticeable B1 grammar slips (e.g., article errors, occasional word-order slips), if the candidate answered all turns intelligibly and asked questions back, they **PASS (15–21 points)** per official Telc B1 standards.
     * **0–14 pts ("Not yet B1"):** Reserve failing scores for incomplete/aborted exams (e.g., only 1–3 turns spoken), zero back-and-forth questions, or severe unintelligibility.`

	EvaluationJSONInstruction = "\n\nYou MUST return your response as a valid JSON object matching the following structure:\n{\n  \"numerical_score\": \"The numerical score (e.g., 19 / 25)\",\n  \"telc_grade\": \"The Telc grade (MUST be one of: Pass with Distinction, Pass, Not yet B1)\",\n  \"top_3_mistakes\": [\n    \"Original spoken error → Corrected B1 German sentence (Brief reason in English)\"\n  ],\n  \"interaction_feedback\": \"Detailed, encouraging feedback in English on dialogue flow, back-questions asked, and topic coverage\",\n  \"foreigner_success_tip\": \"One specific, actionable B1 speaking tip in English with German example phrases\"\n}\nDo not include any other text, markdown formatting, or explanations outside the JSON."
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
