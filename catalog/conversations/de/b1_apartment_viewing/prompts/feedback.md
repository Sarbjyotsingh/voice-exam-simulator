# Conversation Helper Coaching Feedback

Evaluate the learner's German conversation in `{{scenario_title}}` at the `{{level}}` level.
Return a JSON object with:
- `numerical_score` (e.g. `"8 / 10"`)
- `telc_grade` (`"Pass with Distinction"`, `"Pass"`, or `"Keep Practicing"`)
- `top_3_mistakes` (up to 3 spoken German corrections formatted as `"Error → Correction (Brief English reason)"`)
- `interaction_feedback` (2–3 encouraging sentences in English about their conversational flow)
- `foreigner_success_tip` (2–3 useful German B1 phrases for this everyday situation)

TRANSCRIPT:
{{conversation_text}}
