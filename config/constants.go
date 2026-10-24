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

	// Groq Whisper initial_prompt to bias STT towards Telc B1 speaking vocabulary and reduce phonetic hallucinations (must be <= 896 chars)
	GroqWhisperPrompt = "telc Deutsch B1, Teil 1, Teil 2, Teil 3. Wollen wir du oder Sie sagen? Vorname, Familienname, buchstabieren, WG, Softwareentwickler, Muttersprache, Integrationskurs, Hobbys, spazieren gehen. Auf meinem Zettel steht, Hannes Leivig, Bettina Sommer, Lena Becker, Markus Weber, Sabine Lindner, Thomas Keller, Julia Neumann, Tobias Berg, Ilona Schmitz, Stefan Baum, Alex Müller, Ayşe Özel, Maria Lupo, Luca Giotto, Max Krüger, Melanie Roth. Meiner Meinung nach, ich stimme dir zu, Vorteil, Nachteil, Heimatland, Biomarkt, Fast Food, Verkehrsmittel, Deutschlandticket, Pauschalreise, Rucksackreise, Online-Shopping, Großstadt, Dorf, soziale Medien, Fitnessstudio, Rauchverbot. Und du? Wie siehst du das? Ich schlage vor, wie wäre es mit, Abschiedsparty, Börek, Baumarkt, Kleintransporter, Chiemsee, Picknickdecke, Krankenbesuch, Genesungskarte, Filmabend, Aufgaben aufteilen."

	// Default System Prompts
	DefaultSystemPrompt = "You are a helpful German language speaking partner."

	// LLM JSON Instruction
	JSONInstruction = "\n\nYou MUST return your response as a valid JSON object containing an array called 'parts' and an optional boolean key 'exam_ended'. Each item in the array must be an object with optional keys: 'text', 'audio_id', 'speaker', and 'role'. Do not include any other markdown formatting."

	// EvaluationGuardrails enforces STT-aware error reconstruction, official Telc Formblatt M10 grading, and Baukasten-based tips on the backend.
	EvaluationGuardrails = `

### CRITICAL STT, FORMBLATT M10 & BAUKASTEN EVALUATION GUARDRAILS (HIGHEST PRIORITY — OVERRIDES ALL OTHER INSTRUCTIONS):
1. **LANGUAGE REQUIREMENT:** Write ALL evaluation fields ("interaction_feedback", "foreigner_success_tip", and any explanatory notes) in **clear, encouraging English**, while keeping German example phrases, Redemittel, and sentence corrections in **German**.
2. **SPEECH-TO-TEXT (STT) PHONETIC RECONSTRUCTION (CRITICAL):**
   - The Candidate's lines were transcribed automatically by Whisper Speech-to-Text (STT) from a non-native speaker's audio.
   - Whisper frequently produces **phonetic hallucinations** (garbled German words that sound similar to what the learner actually said). Common examples across Teil 1 & Teil 2:
     * "und willkommen zu" / "woher kommst zu" / "was denkst zu" / "wie siehst zu das" -> The learner actually asked **"Und woher kommst du?"** / **"Was denkst du?"** / **"Wie siehst du das?"**.
     * "eine Software in Quickler arbeiten" -> The learner actually said **"als Softwareentwickler arbeiten"** ("Entwickler" misheard as "in Quickler").
     * "Mein Name ist Mutterschweiger" -> The learner was answering "Was ist deine Muttersprache?" ("Meine Muttersprache ist...").
     * "Fettiggerichte" / "obergesunde Ernährung" / "Chick meal" -> The learner said **"Fertiggerichte"** / **"über gesunde Ernährung"** / **"Cheat Meal"**.
     * "Sarab" / "Saurabh" / "Hannes leivig" / "Lena Bäcker" -> Proper names.
   - **YOUR MANDATE:** Always infer the **intended spoken German phrase** from phonetic similarity and conversational context (especially the partner's preceding question and subsequent reaction/answer):
     * **NEVER** treat an STT phonetic hallucination literally (e.g., NEVER tell the learner to say "bei Quickler" or "ich bin hier, um Sie kennenzulernen" when they meant "als Softwareentwickler" or "Und woher kommst du?").
     * If the learner's intended phrase was grammatically correct, **DO NOT list it as a mistake** and **give full credit**!
     * If the learner made a real spoken grammar error alongside an STT artifact (e.g., verb in 2nd position after "weil"/"dass", missing "als" before profession, wrong gender like "Meine Familienname"), reconstruct the **true intended target sentence** in your correction.
3. **NEVER PENALIZE TYPING / PUNCTUATION / CAPITALIZATION:**
   - A human cannot speak capital letters or commas!
   - **STRICTLY FORBIDDEN in top_3_mistakes:** NEVER criticize lowercase words (e.g., "hindi, englisch und deutsch"), comma splices, missing periods, or hyphenation.
4. **WHAT BELONGS IN "top_3_mistakes":**
   - Only include up to 3 **actual spoken German grammar, vocabulary, or syntax mistakes** made by the ` + "`Candidate`" + ` (e.g., verb not at the end in ` + "`dass/weil/obwohl/wenn`" + ` clauses, missing inversion after ` + "`Meiner Meinung nach / Deshalb`" + `, wrong preposition/case like ` + "`aus Türkei`" + ` instead of ` + "`aus der Türkei`" + `, missing ` + "`als`" + ` before a job, wrong collocation like ` + "`Spaziergang gehen`" + ` instead of ` + "`spazieren gehen`" + `, or broken clause structure like ` + "`Ich habe meine Frau wohnt mit mir`" + `).
   - Format each item strictly as: ` + "`\"[Spoken error] → [Natural B1 German correction] ([Short 3-6 word reason in English])\"`" + `.
   - Do NOT put "missing topics" or "did not answer Du/Sie" in ` + "`top_3_mistakes`" + `. Discuss topic coverage and turn-taking in ` + "`interaction_feedback`" + `.
5. **OFFICIAL TELC B1 FORMBLATT M10 SCORING & TEIL WEIGHTS:**
   - **Official Max Score per Teil (75 Points Total Speaking Exam):**
     * **Teil 1 (Kontaktaufnahme):** Max **15 points** (Passwith Distinction: 13–15 pts | Pass: 9–12 pts [>=60%] | Not yet B1: 0–8 pts).
     * **Teil 2 (Gespräch über ein Thema):** Max **30 points** (Pass with Distinction: 26–30 pts | Pass: 18–25 pts [>=60%] | Not yet B1: 0–17 pts).
     * **Teil 3 (Gemeinsam etwas planen):** Max **30 points** (Pass with Distinction: 26–30 pts | Pass: 18–25 pts [>=60%] | Not yet B1: 0–17 pts).
     * *(Use the exact ` + "`max_score`" + ` specified in the prompt — e.g. ` + "`/ 15`" + ` for Teil 1, ` + "`/ 30`" + ` for Teil 2).*
   - **The 4 Official Formblatt M10 Parameters (Grade each with [A]=Voll erfüllt 100%, [B]=Weitgehend erfüllt ~75%, [C]=Teilweise erfüllt ~50%, [D]=Nicht erfüllt 0%):**
     1. **Aufgabenbewältigung & Interaktion** (Teil 1: Ping-pong Q&A + Examiner follow-up; Teil 2: Phase A stimulus text summary [Name, Age, Job, Opinion/Reason] + Phase B personal opinion, habits in Germany, Home Country comparison & back-questions; Teil 3: Collaborative planning across Wann/Wo/Essen-Mitbringsel/Aufgabenverteilung, making proposals, reacting/compromising, and confirming the final plan).
     2. **Ausdrucksfähigkeit** (B1 vocabulary range & Redemittel connectors).
     3. **Formale Richtigkeit** (Spoken grammar: verb-at-end in subordinate clauses, verb-second inversion, prepositions/cases; minor errors that do NOT block comprehension still earn [B]!).
     4. **Aussprache & Flüssigkeit** (Intelligibility and natural flow).
   - **Simulator Protections:**
     * **Du/Sie Rule (Teil 1):** Since the partner already proposes *"du"* and immediately asks *"Und wie heißt du?"* in ` + "`OPEN_01`" + `, replying directly in the *"du"* form is 100% natural and implicitly accepts *"du"*. NEVER penalize the candidate for not explicitly saying *"Ja, wir können du sagen"*.
     * **AI Partner Turn Pacing:** In this simulator, the AI partner controls when the conversation transitions to the Examiner follow-up or final summary. NEVER penalize the candidate if the AI partner wrapped up before asking an optional sub-dimension; evaluate only the turns that actually occurred.
6. **MANDATORY FORMAT FOR "interaction_feedback" & "foreigner_success_tip":**
   - In ` + "`interaction_feedback`" + `: Start with a compact Formblatt M10 summary line showing the 4 letter grades — e.g., ` + "`\"Formblatt M10 Assessment: Aufgabenbewältigung [A/B/C/D] | Ausdrucksfähigkeit [A/B/C/D] | Formale Richtigkeit [A/B/C/D] | Aussprache & Flüssigkeit [A/B/C/D].\"`" + ` — followed by 2–3 encouraging sentences analyzing their dialogue interaction, question-asking, and task fulfillment.
   - In ` + "`foreigner_success_tip`" + `:
     * **NEVER tell a Teil 1 candidate to prepare a 1–2 minute monologue!** Telc B1 Teil 1 strictly requires a back-and-forth *Frage-Antwort* ping-pong dialogue (monologues are penalized by examiners!).
     * Always provide **2–3 concrete, ready-to-memorize German B1 Sentence Baukasten frames (Redemittel)** tailored to what the candidate needed most in this session:
       - *For Teil 1:* Ping-pong counter-questions (*"Und wie sieht es bei dir aus?"*), *seit* + Dative (*"Ich lerne seit ... Monaten Deutsch"*), *als* + profession (*"Ich arbeite als ..."*), or country articles (*"Ich komme aus der Türkei / aus dem Iran"*).
       - *For Teil 2:* Phase A Stimulus Summary frame (*"In meinem Text geht es um [Name], [Alter], [Beruf]. Er/Sie sagt, dass ..., weil ..."*), Phase B Opinion/Nuance frames (*"Meiner Meinung nach ist ..."*, *"Das stimmt zwar, aber ..."*, *"Einerseits ..., andererseits ..."*), and the high-scoring Home Country Comparison (*"In meinem Heimatland ist die Situation ganz anders als in Deutschland, weil ..."*).
       - *For Teil 3:* Making proposals (*"Ich schlage vor, dass wir uns am ... um ... Uhr treffen."*, *"Wie wäre es, wenn wir ...?"*), polite counter-proposals & weather backups (*"Da habe ich leider keine Zeit, weil ... Passt dir stattdessen ...?"*, *"Falls es regnet, könnten wir ..."*), and fair task division (*"Wenn du dich um ... kümmerst, dann besorge ich ..."*).`

	EvaluationJSONInstruction = "\n\nYou MUST return your response as a valid JSON object matching the following structure:\n{\n  \"numerical_score\": \"The numerical score matching the prompt's max_score (e.g., '12 / 15' for Teil 1 or '24 / 30' for Teil 2)\",\n  \"telc_grade\": \"The Telc grade (MUST be one of: Pass with Distinction, Pass, Not yet B1)\",\n  \"top_3_mistakes\": [\n    \"Original spoken error → Corrected B1 German sentence (Brief reason in English)\"\n  ],\n  \"interaction_feedback\": \"Start with 'Formblatt M10 Assessment: Aufgabenbewältigung [Grade] | Ausdrucksfähigkeit [Grade] | Formale Richtigkeit [Grade] | Aussprache & Flüssigkeit [Grade].' followed by encouraging English feedback on dialogue flow, back-questions, and task fulfillment\",\n  \"foreigner_success_tip\": \"One specific, actionable B1 Telc strategy in English with 2-3 exact German Sentence Baukasten (Redemittel) templates to memorize\"\n}\nDo not include any other text, markdown formatting, or explanations outside the JSON."
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
