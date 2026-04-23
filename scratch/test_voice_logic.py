import json
import re

# Simulation of logic in websocket.py
SPEAKER_TAG_REGEX = re.compile(r'^([\w\s\d]+):\s*')

def test_voice_logic(init_data, llm_response_sentences):
    voice_mapping = init_data.get("voice_mapping", {})
    normalized_mapping = {k.lower(): v for k, v in voice_mapping.items()}
    
    session_default_voice_id = init_data.get("voice_id")
    if not session_default_voice_id:
        for key in ["default", "examiner", "assistant"]:
            if key in normalized_mapping:
                session_default_voice_id = normalized_mapping[key]
                break
    
    if not session_default_voice_id and len(normalized_mapping) == 1:
        session_default_voice_id = list(normalized_mapping.values())[0]

    print(f"Session Default: {session_default_voice_id}")
    
    current_voice_id = session_default_voice_id
    
    for sentence in llm_response_sentences:
        clean_sentence = sentence.strip()
        if clean_sentence:
            tag_match = SPEAKER_TAG_REGEX.match(clean_sentence)
            if tag_match:
                role_name = tag_match.group(1).strip()
                role_key = role_name.lower()
                if role_key in normalized_mapping:
                    current_voice_id = normalized_mapping[role_key]
                    print(f"  [SWITCH] Found Role '{role_name}', Switching to: {current_voice_id}")
                else:
                    print(f"  [TAG] Found Role '{role_name}' (Not in Mapping), Using current: {current_voice_id}")
                clean_sentence = clean_sentence[tag_match.end():].strip()
            
            print(f"  [TTS] Sending: '{clean_sentence}' with Voice ID: {current_voice_id}")

print("--- Test Case 1: Matching Role ---")
test_voice_logic(
    {"voice_mapping": {"Examiner": "voice_1"}},
    ["Examiner: Hello.", "How are you?"]
)

print("\n--- Test Case 2: Case Insensitivity ---")
test_voice_logic(
    {"voice_mapping": {"examiner": "voice_1"}},
    ["Examiner: Hello."]
)

print("\n--- Test Case 3: Session Default (Only one entry) ---")
test_voice_logic(
    {"voice_mapping": {"AI": "voice_ai"}},
    ["Hello without tag."]
)

print("\n--- Test Case 4: Explicit voice_id ---")
test_voice_logic(
    {"voice_id": "explicit_id", "voice_mapping": {"Other": "voice_2"}},
    ["Hello."]
)

print("\n--- Test Case 5: Role not in mapping ---")
test_voice_logic(
    {"voice_mapping": {"Examiner": "voice_1"}},
    ["Unknown: Hello."]
)
