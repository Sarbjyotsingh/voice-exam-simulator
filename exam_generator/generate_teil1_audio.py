#!/usr/bin/env python3
"""
Telc B1 Speaking Exam (Teil 1: Kontaktaufnahme) — Audio & S3 Manifest Generator
Generates:
- 5 Distinct Examiners with custom voices and scripts:
  - B1_S01: Herr Wagner (Male: Jvf6TAXwMUVTSR20U0f9)
  - B1_S02: Frau Fischer (Female: MGG5Irb57ATHvyIeTEYo)
  - B1_S03: Frau Erdoğan (Female: zKHQdbB8oaQ7roNTiDTK)
  - B1_S04: Herr Richter (Male: z1EhmmPwF0ENGYE8dBE6)
  - B1_S05: Frau Dr. Meyer (Female: 4YKI1ONIhgWEKHyAHYPC)
- 5 Distinct Partner Personas (Leyla, Omar, Elena, Mateo, Janusz) — 100% keeping the "du" opening sentence!
- Populates output_s3/ and upload_package/telc/b1/
"""

import json
import os
import shutil
import sys
import time
import urllib.error
import urllib.request
from concurrent.futures import ThreadPoolExecutor, as_completed
from pathlib import Path

BASE_DIR = Path(__file__).resolve().parent
PROJECT_ROOT = BASE_DIR.parent
ENV_PATH = PROJECT_ROOT / ".env"
OUTPUT_DIR = BASE_DIR / "output_s3"
UPLOAD_ROOT = BASE_DIR / "upload_package" / "telc" / "b1"
CACHE_AUDIO_DIR = BASE_DIR / "audio_cache_teil1"

ELEVENLABS_BASE_URL = "https://api.elevenlabs.io/v1/text-to-speech"
ELEVENLABS_MODEL = "eleven_flash_v2_5"

EXAMINERS = [
    {
        "id": "EX1",
        "name": "Herr Wagner",
        "voice_id": "Jvf6TAXwMUVTSR20U0f9",
        "intro": (
            "Guten Tag. Herzlich willkommen zur mündlichen telc B1 Prüfung. Mein Name ist Herr Wagner und das ist mein Kollege Herr Weber. Wir beginnen heute mit Teil eins: dem gegenseitigen Kennenlernen. Bitte stellen Sie sich im Dialog vor und stellen Sie sich gegenseitig Fragen."
        ),
        "follow_01": (
            "Vielen Dank Ihnen beiden. Dann stelle ich auch noch eine kurze Frage an Sie: Können Sie mir bitte Ihren Familiennamen buchstabieren?"
        ),
        "follow_02": (
            "Danke schön. Ich hätte noch eine kurze Frage an Sie: Wo und wie lange haben Sie denn schon Deutsch gelernt?"
        ),
        "int_topic": (
            "Bitte bleiben Sie beim Thema des ersten Prüfungsteils und unterhalten Sie sich mit Ihrem Partner über die Punkte zur Person."
        ),
        "int_polite": (
            "Bitte bleiben Sie höflich und sachlich im Rahmen dieser Prüfung."
        ),
        "int_dialog": (
            "Darf ich kurz unterbrechen: Bitte führen Sie in Teil eins ein Gespräch im Dialog. Stellen Sie auch Ihrem Partner eine Frage."
        ),
        "outro": (
            "Sehr schön, vielen Dank für das nette Gespräch. Damit schließen wir den ersten Teil der Prüfung ab."
        ),
    },
    {
        "id": "EX2",
        "name": "Frau Fischer",
        "voice_id": "MGG5Irb57ATHvyIeTEYo",
        "intro": (
            "Schönen guten Tag und herzlich willkommen zur mündlichen Prüfung telc Deutsch B1. Mein Name ist Ellen Fischer und das ist meine Kollegin Frau Bergmann. Die Prüfung besteht aus drei Teilen und wir starten gleich mit Teil eins: Bitte lernen Sie sich im Dialog kennen. Fangen Sie bitte an."
        ),
        "follow_01": (
            "Danke schön. Ich hätte auch noch eine kurze Frage an Sie: Wo und wie lange haben Sie denn schon Deutsch gelernt?"
        ),
        "follow_02": (
            "Vielen Dank. Eine kurze Frage noch von meiner Seite: Was möchten Sie beruflich in Zukunft hier in Deutschland machen?"
        ),
        "int_topic": (
            "Bitte bleiben Sie bei den Themen von Teil eins und sprechen Sie über Ihre persönlichen Erfahrungen."
        ),
        "int_polite": (
            "Bitte achten Sie auf einen höflichen und angemessenen Ton während der Prüfung."
        ),
        "int_dialog": (
            "Entschuldigung, denken Sie bitte daran, dass dies ein Dialog ist. Geben Sie auch Ihrem Partner die Möglichkeit zu sprechen."
        ),
        "outro": (
            "Prima, vielen Dank Ihnen beiden! Das war schon Teil eins. Dann kommen wir jetzt zum zweiten Teil der Prüfung."
        ),
    },
    {
        "id": "EX3",
        "name": "Frau Erdoğan",
        "voice_id": "zKHQdbB8oaQ7roNTiDTK",
        "intro": (
            "Hallo und herzlich willkommen zu unserer mündlichen Prüfung telc B1. Mein Name ist Frau Erdoğan, neben mir sitzt mein lieber Kollege Herr Özcan. Im ersten Teil lernen Sie sich kennen – bitte im Dialog, also immer Frage und Antwort. Bitte schön."
        ),
        "follow_01": (
            "Vielen Dank. Dann stelle ich Ihnen auch noch eine Frage: Was machen Sie denn besonders gern am Wochenende in Ihrer Freizeit?"
        ),
        "follow_02": (
            "Danke sehr. Eine kurze Frage noch: Gefällt es Ihnen in Ihrem Wohnort, oder möchten Sie später einmal umziehen?"
        ),
        "int_topic": (
            "Bitte kehren Sie zum Thema zurück und sprechen Sie über die vorgegebenen Stichpunkte."
        ),
        "int_polite": (
            "Wir bitten Sie, im Rahmen dieser Prüfung stets respektvoll und sachlich zu bleiben."
        ),
        "int_dialog": (
            "Kurzer Hinweis: Bitte wechseln Sie sich ab. Ein Partner fragt, der andere antwortet."
        ),
        "outro": (
            "Sehr gut, vielen Dank! Teil eins ist damit beendet und wir gehen direkt über zu Teil zwei."
        ),
    },
    {
        "id": "EX4",
        "name": "Herr Richter",
        "voice_id": "z1EhmmPwF0ENGYE8dBE6",
        "intro": (
            "Guten Tag! Ich begrüße Sie ganz herzlich zur mündlichen telc B1 Prüfung. Mein Name ist Herr Richter und das ist meine Kollegin Frau Schwarz. Wir beginnen mit dem ersten Teil: Bitte unterhalten Sie sich und stellen Sie sich einander vor. Bitte beginnen Sie."
        ),
        "follow_01": (
            "Danke sehr. Eine kurze Zusatzfrage hätte ich noch an Sie: Was möchten Sie beruflich in Zukunft hier in Deutschland machen?"
        ),
        "follow_02": (
            "Vielen Dank Ihnen beiden. Eine kurze Frage noch: Können Sie Ihren Nachnamen bitte einmal buchstabieren?"
        ),
        "int_topic": (
            "Bitte bleiben Sie beim Thema und führen Sie das Gespräch über die persönlichen Punkte fort."
        ),
        "int_polite": (
            "Bitte wahren Sie die Höflichkeit im Prüfungsgespräch."
        ),
        "int_dialog": (
            "Bitte denken Sie daran, einander Fragen zu stellen und nicht nur alleine zu sprechen."
        ),
        "outro": (
            "Vielen Dank Ihnen beiden, das war der erste Teil. Jetzt machen wir mit Teil zwei weiter."
        ),
    },
    {
        "id": "EX5",
        "name": "Frau Dr. Meyer",
        "voice_id": "4YKI1ONIhgWEKHyAHYPC",
        "intro": (
            "Willkommen zur mündlichen Prüfung telc B1. Ich bin Frau Dr. Meyer und das ist mein Kollege Herr Klein. Die Prüfung besteht aus drei Teilen. Wir fangen mit Teil eins an: Tauschen Sie persönliche Informationen aus und stellen Sie sich gegenseitig Fragen. Bitteschön."
        ),
        "follow_01": (
            "Vielen Dank. Eine kurze Frage noch von meiner Seite: Gefällt es Ihnen in Ihrem Wohnort in Deutschland, oder möchten Sie später umziehen?"
        ),
        "follow_02": (
            "Danke schön. Eine Frage noch: Wie gefällt Ihnen das Deutschlernen bisher und wo haben Sie gelernt?"
        ),
        "int_topic": (
            "Bitte konzentrieren Sie sich auf die Themen zur Person für diesen ersten Prüfungsteil."
        ),
        "int_polite": (
            "Bitte bleiben Sie sachlich und höflich im Gespräch."
        ),
        "int_dialog": (
            "Bitte stellen Sie Ihrem Prüfungspartner auch Fragen, damit ein echtes Gespräch entsteht."
        ),
        "outro": (
            "Wunderbar, vielen Dank! Damit haben Sie Teil eins geschafft. Kommen wir nun zu Teil zwei."
        ),
    },
]

PERSONAS = [
    {
        "sample_id": "B1_S01",
        "sample_name": "B1 Sample Paper 01 (Leyla)",
        "folder": "B1_Exam_Sample_01",
        "code": "LEY",
        "speaker": "Leyla",
        "full_name": "Leyla Yılmaz",
        "voice_id": "NE7AIW5DoJ7lUosXV2KR",
        "examiner_idx": 0,
        "answers": {
            "OPEN_01": (
                "Opening & Greeting",
                "question",
                "Hallo! Wollen wir du sagen? Ich fange einfach mal an: Ich heiße Leyla Yılmaz. Und wie heißt du?",
            ),
            "NAME_A1": (
                "Name & Surname",
                "answer",
                "Mein Vorname ist Leyla und mein Nachname ist Yılmaz.",
            ),
            "AGE_A1": (
                "Age",
                "answer",
                "Ich bin neunundzwanzig Jahre alt.",
            ),
            "ORIG_A1": (
                "Origin",
                "answer",
                "Ich komme ursprünglich aus der Türkei, aus Izmir. Das ist eine schöne große Stadt direkt am Meer.",
            ),
            "RES_A1": (
                "Residence",
                "answer",
                "Ich lebe seit fast zwei Jahren in Deutschland und wohne in Münster. Wir haben dort eine gemütliche Dreizimmerwohnung.",
            ),
            "FAM_A1": (
                "Family",
                "answer",
                "Ich bin verheiratet und habe zwei kleine Kinder, einen Sohn und eine Tochter. Meine Eltern und meine Schwester wohnen aber noch in der Türkei.",
            ),
            "WORK_A1": (
                "Work & Education",
                "answer",
                "Von Beruf bin ich Grundschullehrerin. Zurzeit arbeite ich in Teilzeit an einer Schule hier in Münster.",
            ),
            "LANG_A1": (
                "Languages",
                "answer",
                "Meine Muttersprache ist Türkisch. Außerdem spreche ich gut Englisch und natürlich lerne ich jeden Tag Deutsch.",
            ),
            "GER_A1": (
                "Learning German",
                "answer",
                "Ich habe hier in Münster an der Volkshochschule einen Integrationskurs gemacht, weil ich für meine Arbeit als Lehrerin gut Deutsch sprechen muss.",
            ),
            "HOB_A1": (
                "Hobbies & Free Time",
                "answer",
                "In meiner Freizeit gehe ich sehr gerne mit meinen Kindern im Park spazieren, ich fahre viel Fahrrad und am Wochenende koche ich gerne für meine Familie.",
            ),
        },
    },
    {
        "sample_id": "B1_S02",
        "sample_name": "B1 Sample Paper 02 (Omar)",
        "folder": "B1_Exam_Sample_02",
        "code": "OMA",
        "speaker": "Omar",
        "full_name": "Omar Al-Masri",
        "voice_id": "A2eYGaZmwykOiEVBlQ7h",
        "examiner_idx": 1,
        "answers": {
            "OPEN_01": (
                "Opening & Greeting",
                "question",
                "Hallo! Können wir ruhig du sagen? Ich heiße Omar Al-Masri. Und wie ist dein Name?",
            ),
            "NAME_A1": (
                "Name & Surname",
                "answer",
                "Ich heiße Omar, und mein Familienname ist Al-Masri.",
            ),
            "AGE_A1": (
                "Age",
                "answer",
                "Ich bin fünfundzwanzig Jahre alt.",
            ),
            "ORIG_A1": (
                "Origin",
                "answer",
                "Ich komme aus Syrien, aus der Hauptstadt Damaskus.",
            ),
            "RES_A1": (
                "Residence",
                "answer",
                "Momentan wohne ich in Berlin in einer Wohngemeinschaft mit zwei Freunden. Aber ich suche gerade eine eigene kleine Wohnung.",
            ),
            "FAM_A1": (
                "Family",
                "answer",
                "Ich bin noch ledig und habe keine Kinder. Meine Eltern und meine zwei Brüder leben noch in Syrien.",
            ),
            "WORK_A1": (
                "Work & Education",
                "answer",
                "In Syrien habe ich als Buchhalter gearbeitet. Jetzt mache ich hier in Berlin eine Ausbildung als Kaufmann für Büromanagement.",
            ),
            "LANG_A1": (
                "Languages",
                "answer",
                "Meine Muttersprache ist Arabisch. Außerdem spreche ich Englisch, ein wenig Französisch und jetzt natürlich Deutsch.",
            ),
            "GER_A1": (
                "Learning German",
                "answer",
                "Ich lerne seit einem Jahr Deutsch in einer Sprachschule in Berlin. Für meine Ausbildung im Büro brauche ich das B1-Zertifikat.",
            ),
            "HOB_A1": (
                "Hobbies & Free Time",
                "answer",
                "Nach der Arbeit gehe ich regelmäßig ins Fitnessstudio, ich spiele Fußball im Verein und am Wochenende treffe ich mich gern mit Freunden im Café.",
            ),
        },
    },
    {
        "sample_id": "B1_S03",
        "sample_name": "B1 Sample Paper 03 (Elena)",
        "folder": "B1_Exam_Sample_03",
        "code": "ELE",
        "speaker": "Elena",
        "full_name": "Elena Kovalenko",
        "voice_id": "gVOibprogMfmHVVyo5r6",
        "examiner_idx": 2,
        "answers": {
            "OPEN_01": (
                "Opening & Greeting",
                "question",
                "Guten Tag! Wollen wir uns duzen? Mein Name ist Elena Kovalenko. Und wie heißt du?",
            ),
            "NAME_A1": (
                "Name & Surname",
                "answer",
                "Mein Vorname ist Elena und mein Nachname ist Kovalenko.",
            ),
            "AGE_A1": (
                "Age",
                "answer",
                "Ich bin vierunddreißig Jahre alt.",
            ),
            "ORIG_A1": (
                "Origin",
                "answer",
                "Ich komme aus der Ukraine, aus der Stadt Charkiw.",
            ),
            "RES_A1": (
                "Residence",
                "answer",
                "Zurzeit wohne ich in München. Wir haben dort eine helle Wohnung mit drei Zimmern und einem kleinen Balkon.",
            ),
            "FAM_A1": (
                "Family",
                "answer",
                "Ich bin verheiratet und wir haben einen Sohn. Er ist acht Jahre alt und geht hier in die Grundschule.",
            ),
            "WORK_A1": (
                "Work & Education",
                "answer",
                "Ich bin Krankenschwester von Beruf. Im Moment mache ich ein Praktikum im Krankenhaus, weil ich auf die Anerkennung von meinem Diplom warte.",
            ),
            "LANG_A1": (
                "Languages",
                "answer",
                "Meine Muttersprache ist Ukrainisch, aber ich spreche auch fließend Russisch, ein bisschen Englisch und jetzt Deutsch.",
            ),
            "GER_A1": (
                "Learning German",
                "answer",
                "Zuerst habe ich drei Monate online im Internet Deutsch gelernt und danach einen Intensivkurs in München besucht.",
            ),
            "HOB_A1": (
                "Hobbies & Free Time",
                "answer",
                "In meiner Freizeit lese ich sehr gerne Bücher, ich male gern und am Wochenende gehe ich oft mit meinem Sohn schwimmen.",
            ),
        },
    },
    {
        "sample_id": "B1_S04",
        "sample_name": "B1 Sample Paper 04 (Mateo)",
        "folder": "B1_Exam_Sample_04",
        "code": "MAT",
        "speaker": "Mateo",
        "full_name": "Mateo Morales",
        "voice_id": "1J0wWp4zPQIvsK7Xwh34",
        "examiner_idx": 3,
        "answers": {
            "OPEN_01": (
                "Opening & Greeting",
                "question",
                "Hallo! Lass uns gerne du sagen. Ich bin Mateo Morales. Und wie heißt du?",
            ),
            "NAME_A1": (
                "Name & Surname",
                "answer",
                "Ich heiße Mateo, und mein Nachname ist Morales.",
            ),
            "AGE_A1": (
                "Age",
                "answer",
                "Ich bin siebenundzwanzig Jahre alt.",
            ),
            "ORIG_A1": (
                "Origin",
                "answer",
                "Ich komme aus Spanien und bin in der Hauptstadt Madrid geboren.",
            ),
            "RES_A1": (
                "Residence",
                "answer",
                "Ich wohne seit einem Jahr in Frankfurt am Main, allein in einer kleinen Zweizimmerwohnung in der dritten Etage.",
            ),
            "FAM_A1": (
                "Family",
                "answer",
                "Ich bin ledig und wohne hier allein. Meine Eltern und meine jüngere Schwester leben alle noch in Madrid.",
            ),
            "WORK_A1": (
                "Work & Education",
                "answer",
                "Ich bin Informatiker von Beruf und arbeite als Softwareentwickler bei einer Firma hier in Frankfurt.",
            ),
            "LANG_A1": (
                "Languages",
                "answer",
                "Meine Muttersprache ist Spanisch. Außerdem spreche ich fließend Englisch, ein bisschen Italienisch und Deutsch.",
            ),
            "GER_A1": (
                "Learning German",
                "answer",
                "Ich habe schon in Spanien an der Universität ein bisschen Deutsch gelernt und besuche jetzt abends nach der Arbeit einen Sprachkurs.",
            ),
            "HOB_A1": (
                "Hobbies & Free Time",
                "answer",
                "In meiner Freizeit spiele ich gerne Gitarre, ich gehe oft am Fluss joggen und abends schaue ich gerne Filme.",
            ),
        },
    },
    {
        "sample_id": "B1_S05",
        "sample_name": "B1 Sample Paper 05 (Janusz)",
        "folder": "B1_Exam_Sample_05",
        "code": "JAN",
        "speaker": "Janusz",
        "full_name": "Janusz Kamiński",
        "voice_id": "K5ZVtkkBnuPY6YqXs70E",
        "examiner_idx": 4,
        "answers": {
            "OPEN_01": (
                "Opening & Greeting",
                "question",
                "Guten Tag! Du kannst mich gerne duzen. Ich heiße Janusz Kamiński. Wie ist dein Name?",
            ),
            "NAME_A1": (
                "Name & Surname",
                "answer",
                "Mein Vorname ist Janusz und mein Familienname ist Kamiński.",
            ),
            "AGE_A1": (
                "Age",
                "answer",
                "Ich bin achtunddreißig Jahre alt.",
            ),
            "ORIG_A1": (
                "Origin",
                "answer",
                "Ich komme aus Polen, aus der Nähe von Krakau.",
            ),
            "RES_A1": (
                "Residence",
                "answer",
                "Ich wohne jetzt seit drei Jahren in Essen, im Ruhrgebiet. Wir wohnen dort in einem kleinen Haus mit Garten.",
            ),
            "FAM_A1": (
                "Family",
                "answer",
                "Ich bin verheiratet und habe zwei Kinder: eine Tochter, sie ist zehn, und einen Sohn, er ist sechs Jahre alt.",
            ),
            "WORK_A1": (
                "Work & Education",
                "answer",
                "Von Beruf bin ich Elektriker und arbeite Vollzeit in einer großen Firma, die Autoteile herstellt.",
            ),
            "LANG_A1": (
                "Languages",
                "answer",
                "Meine Muttersprache ist Polnisch. Ich spreche auch etwas Englisch, ein bisschen Russisch und natürlich Deutsch.",
            ),
            "GER_A1": (
                "Learning German",
                "answer",
                "Am Anfang habe ich Deutsch direkt bei der Arbeit mit meinen Kollegen gelernt, und seit sechs Monaten mache ich noch einen Abendkurs.",
            ),
            "HOB_A1": (
                "Hobbies & Free Time",
                "answer",
                "Am Wochenende arbeite ich gern im Garten, im Sommer grillen wir oft mit Freunden und ich schaue sehr gerne Fußball.",
            ),
        },
    },
]

EXPRESSIONS = [
    ("EXP_01", "Reaction: Curious / Interested", "Ah, interessant!"),
    ("EXP_02", "Reaction: Warm / Positive", "Oh, schön!"),
    ("EXP_03", "Reaction: Upbeat / Encouraging", "Super!"),
    ("EXP_04", "Reaction: Enthusiastic", "Das klingt ja toll!"),
    ("EXP_05", "Reaction: Empathetic / Understanding", "Ach so, ich verstehe."),
    ("EXP_06", "Reaction: Surprised / Impressed", "Oh, wirklich? Das ist ja spannend!"),
    ("EXP_07", "Reaction: Casual Acknowledgment", "Alles klar!"),
    ("EXP_08", "Reaction: Relatable", "Ja, das kenne ich gut."),
    ("EXP_09", "Reaction: Casual / Friendly", "Cool!"),
    ("EXP_10", "Reaction: Neutral Transition", "Ah, okay!"),
    ("EXP_11", "Reaction: Polite Pleasure", "Das freut mich!"),
    ("EXP_12", "Reaction: Shared Experience", "Oh, ich auch!"),
    ("EXP_13", "Reaction: Agreeing", "Ja, das stimmt."),
    ("EXP_14", "Reaction: Supportive / Good Luck", "Ich wünsche dir viel Erfolg dabei!"),
    ("EXP_15", "Reaction: You're Welcome", "Gerne! Kein Problem."),
    ("EXP_16", "Reaction: Sympathetic", "Oh, das tut mir leid."),
    ("EXP_17", "Reaction: Encouraging", "Sehr gut!"),
    ("EXP_18", "Reaction: Reciprocation", "Danke, gleichfalls!"),
]

SHARED_PARTNER_LINES = [
    ("DUSIE_Q1", "Du/Sie Agreement", "question", "Hallo! Wollen wir du oder Sie sagen?"),
    ("DUSIE_A1", "Du/Sie Agreement", "answer", "Ja, gerne! Wir können sehr gerne du sagen."),
    ("NAME_Q1", "Name & Surname", "question", "Wie heißt du? Und wie ist dein Nachname?"),
    ("NAME_Q2", "Surname Follow-up", "question", "Und wie ist dein Familienname?"),
    ("AGE_Q1", "Age", "question", "Und wie alt bist du, wenn ich fragen darf?"),
    ("ORIG_Q1", "Origin", "question", "Woher kommst du eigentlich?"),
    ("ORIG_Q2", "Hometown Follow-up", "question", "Aus welcher Stadt kommst du genau?"),
    ("RES_Q1", "Residence", "question", "Wo wohnst du jetzt hier in Deutschland?"),
    ("RES_Q2", "Housing Situation", "question", "Wohnst du in einer Wohnung, in einem Haus oder in einer WG?"),
    ("FAM_Q1", "Family", "question", "Lebst du allein hier oder hast du Familie in Deutschland?"),
    ("FAM_Q2", "Marital Status & Children", "question", "Bist du verheiratet und hast du Kinder?"),
    ("WORK_Q1", "Work & Education", "question", "Was machst du beruflich? Arbeitest du schon oder machst du eine Ausbildung?"),
    ("LANG_Q1", "Languages", "question", "Welche Sprachen sprichst du? Was ist deine Muttersprache?"),
    ("GER_Q1", "Learning German", "question", "Wo und wie lange hast du schon Deutsch gelernt?"),
    ("HOB_Q1", "Hobbies & Free Time", "question", "Was machst du gern in deiner Freizeit? Hast du Hobbys?"),
    ("BACK_Q1", "Counter-Question: Und du?", "question", "Und du?"),
    ("BACK_Q2", "Counter-Question: Und bei dir?", "question", "Und wie ist das bei dir?"),
    ("CLAR_Q1", "Clarification", "question", "Entschuldigung, kannst du das bitte noch einmal wiederholen? Ich habe dich akustisch nicht ganz verstanden."),
    ("SIL_Q1", "Silence Recovery", "question", "Hallo? Bist du noch da? Wollen wir weitermachen?"),
    ("CLOSE_01", "Partner Closing", "dialogue", "Vielen Dank für das nette Gespräch! Es hat mich sehr gefreut, dich kennenzulernen."),
]


def load_api_key() -> str:
    api_key = os.environ.get("ELEVENLABS_API_KEY", "").strip()
    if api_key:
        return api_key
    if ENV_PATH.exists():
        for line in ENV_PATH.read_text(encoding="utf-8").splitlines():
            line = line.strip()
            if line.startswith("ELEVENLABS_API_KEY="):
                return line.split("=", 1)[1].strip().strip('"').strip("'")
    raise RuntimeError(f"ELEVENLABS_API_KEY not found in environment or {ENV_PATH}")


def synthesize_clip(api_key: str, voice_id: str, text: str, dest_path: Path) -> str:
    if dest_path.exists() and dest_path.stat().st_size > 512:
        return f"SKIP (cached): {dest_path.name}"

    url = f"{ELEVENLABS_BASE_URL}/{voice_id}"
    payload = {
        "text": text,
        "model_id": ELEVENLABS_MODEL,
        "language_code": "de",
        "voice_settings": {
            "stability": 0.60,
            "similarity_boost": 0.75,
        },
    }
    data = json.dumps(payload).encode("utf-8")
    headers = {
        "Accept": "audio/mpeg",
        "Content-Type": "application/json",
        "xi-api-key": api_key,
    }

    for attempt in range(1, 5):
        try:
            req = urllib.request.Request(url, data=data, headers=headers, method="POST")
            with urllib.request.urlopen(req, timeout=45) as resp:
                audio_bytes = resp.read()
                if len(audio_bytes) < 256:
                    raise RuntimeError(f"Received truncated audio ({len(audio_bytes)} bytes)")
                dest_path.parent.mkdir(parents=True, exist_ok=True)
                dest_path.write_bytes(audio_bytes)
                return f"OK ({len(audio_bytes)} B): {dest_path.name}"
        except urllib.error.HTTPError as e:
            err_body = e.read().decode("utf-8", errors="replace")
            if e.code in (409, 429) and attempt < 4:
                time.sleep(2.0 * attempt)
                continue
            raise RuntimeError(f"HTTP {e.code} for {dest_path.name} (voice {voice_id}): {err_body}")
        except Exception as e:
            if attempt < 4:
                time.sleep(1.5 * attempt)
                continue
            raise RuntimeError(f"Failed {dest_path.name}: {e}")


def build_persona_dialogues(persona: dict, examiner: dict) -> list:
    code = persona["code"]
    speaker = persona["speaker"]
    ans_map = persona["answers"]
    ex_id = examiner["id"]
    ex_name = examiner["name"]

    dialogues = []

    # 1. Custom Examiner Introduction
    dialogues.append(
        {
            "topic": "Exam Introduction",
            "speaker": ex_name,
            "role": "Examiner",
            "interaction": {
                "dialogue": {
                    "audio_id": f"DE_{ex_id}_INTRO_01",
                    "text": examiner["intro"],
                }
            },
        }
    )

    # 2. Partner Opening & Du/Sie Proposal (100% Keeping "du" sentence)
    open_id = f"DE_{code}_OPEN_01"
    dialogues.append(
        {
            "topic": "Opening & Du/Sie Proposal",
            "speaker": speaker,
            "role": "Participant",
            "interaction": {
                "question": {
                    "audio_id": open_id,
                    "text": ans_map["OPEN_01"][2],
                },
                "answer": {
                    "audio_id": f"DE_{code}_DUSIE_A1",
                    "text": "Ja, gerne! Wir können sehr gerne du sagen.",
                },
                "dialogue": {
                    "audio_id": f"DE_{code}_DUSIE_Q1",
                    "text": "Hallo! Wollen wir du oder Sie sagen?",
                },
            },
        }
    )

    # 3. Core 7 Topics + Age + German Learning
    paired_topics = [
        (
            "Name & Surname",
            ("NAME_Q1", "Wie heißt du? Und wie ist dein Nachname?"),
            ("NAME_A1", ans_map["NAME_A1"][2]),
            ("NAME_Q2", "Und wie ist dein Familienname?"),
        ),
        (
            "Age",
            ("AGE_Q1", "Und wie alt bist du, wenn ich fragen darf?"),
            ("AGE_A1", ans_map["AGE_A1"][2]),
            None,
        ),
        (
            "Origin & Hometown",
            ("ORIG_Q1", "Woher kommst du eigentlich?"),
            ("ORIG_A1", ans_map["ORIG_A1"][2]),
            ("ORIG_Q2", "Aus welcher Stadt kommst du genau?"),
        ),
        (
            "Residence & Housing",
            ("RES_Q1", "Wo wohnst du jetzt hier in Deutschland?"),
            ("RES_A1", ans_map["RES_A1"][2]),
            ("RES_Q2", "Wohnst du in einer Wohnung, in einem Haus oder in einer WG?"),
        ),
        (
            "Family & Marital Status",
            ("FAM_Q1", "Lebst du allein hier oder hast du Familie in Deutschland?"),
            ("FAM_A1", ans_map["FAM_A1"][2]),
            ("FAM_Q2", "Bist du verheiratet und hast du Kinder?"),
        ),
        (
            "Work & Education",
            ("WORK_Q1", "Was machst du beruflich? Arbeitest du schon oder machst du eine Ausbildung?"),
            ("WORK_A1", ans_map["WORK_A1"][2]),
            None,
        ),
        (
            "Languages",
            ("LANG_Q1", "Welche Sprachen sprichst du? Was ist deine Muttersprache?"),
            ("LANG_A1", ans_map["LANG_A1"][2]),
            None,
        ),
        (
            "Learning German",
            ("GER_Q1", "Wo und wie lange hast du schon Deutsch gelernt?"),
            ("GER_A1", ans_map["GER_A1"][2]),
            None,
        ),
        (
            "Hobbies & Free Time",
            ("HOB_Q1", "Was machst du gern in deiner Freizeit? Hast du Hobbys?"),
            ("HOB_A1", ans_map["HOB_A1"][2]),
            None,
        ),
    ]

    for topic_name, q_tuple, a_tuple, d_tuple in paired_topics:
        inter = {
            "question": {
                "audio_id": f"DE_{code}_{q_tuple[0]}",
                "text": q_tuple[1],
            },
            "answer": {
                "audio_id": f"DE_{code}_{a_tuple[0]}",
                "text": a_tuple[1],
            },
        }
        if d_tuple is not None:
            inter["dialogue"] = {
                "audio_id": f"DE_{code}_{d_tuple[0]}",
                "text": d_tuple[1],
            }
        dialogues.append(
            {
                "topic": topic_name,
                "speaker": speaker,
                "role": "Participant",
                "interaction": inter,
            }
        )

    # 4. Counter-Questions, Clarification & Closing
    dialogues.append(
        {
            "topic": "Counter-Questions & Closing",
            "speaker": speaker,
            "role": "Participant",
            "interaction": {
                "question": {
                    "audio_id": f"DE_{code}_BACK_Q1",
                    "text": "Und du?",
                },
                "answer": {
                    "audio_id": f"DE_{code}_BACK_Q2",
                    "text": "Und wie ist das bei dir?",
                },
                "dialogue": {
                    "audio_id": f"DE_{code}_CLOSE_01",
                    "text": "Vielen Dank für das nette Gespräch! Es hat mich sehr gefreut, dich kennenzulernen.",
                },
            },
        }
    )

    dialogues.append(
        {
            "topic": "Clarification & Silence Recovery",
            "speaker": speaker,
            "role": "Participant",
            "interaction": {
                "question": {
                    "audio_id": f"DE_{code}_CLAR_Q1",
                    "text": "Entschuldigung, kannst du das bitte noch einmal wiederholen? Ich habe dich akustisch nicht ganz verstanden.",
                },
                "dialogue": {
                    "audio_id": f"DE_{code}_SIL_Q1",
                    "text": "Hallo? Bist du noch da? Wollen wir weitermachen?",
                },
            },
        }
    )

    # 5. Category A: 18 Standalone Modular Expressions
    for i in range(0, len(EXPRESSIONS), 3):
        chunk = EXPRESSIONS[i : i + 3]
        inter = {}
        keys = ["dialogue", "question", "answer"]
        for k, (exp_code, _, exp_text) in zip(keys, chunk):
            inter[k] = {
                "audio_id": f"DE_{code}_{exp_code}",
                "text": exp_text,
            }
        dialogues.append(
            {
                "topic": f"Modular Expressions ({chunk[0][0]}-{chunk[-1][0]})",
                "speaker": speaker,
                "role": "Participant",
                "interaction": inter,
            }
        )

    # 6. Custom Examiner Follow-Up Questions, Interventions & Outro
    examiner_items = [
        (f"DE_{ex_id}_FOLLOW_01", "Examiner Follow-Up 1", examiner["follow_01"]),
        (f"DE_{ex_id}_FOLLOW_02", "Examiner Follow-Up 2", examiner["follow_02"]),
        (f"DE_{ex_id}_INT_TOPIC", "Examiner Intervention: Stay on Topic", examiner["int_topic"]),
        (f"DE_{ex_id}_INT_POLITE", "Examiner Intervention: Politeness", examiner["int_polite"]),
        (f"DE_{ex_id}_INT_DIALOG", "Examiner Intervention: Dialogue Format", examiner["int_dialog"]),
        (f"DE_{ex_id}_OUTRO_01", "Moderation Teil 1 Ende", examiner["outro"]),
    ]

    for clip_id, ex_topic, ex_text in examiner_items:
        dialogues.append(
            {
                "topic": ex_topic,
                "speaker": ex_name,
                "role": "Examiner",
                "interaction": {
                    "dialogue": {
                        "audio_id": clip_id,
                        "text": ex_text,
                    }
                },
            }
        )

    return dialogues


def main():
    api_key = load_api_key()
    CACHE_AUDIO_DIR.mkdir(parents=True, exist_ok=True)
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)

    tasks = []

    # 1. Examiner clips (7 per examiner x 5 = 35 clips)
    for ex in EXAMINERS:
        ex_id = ex["id"]
        vid = ex["voice_id"]
        tasks.append((vid, f"DE_{ex_id}_INTRO_01", ex["intro"]))
        tasks.append((vid, f"DE_{ex_id}_FOLLOW_01", ex["follow_01"]))
        tasks.append((vid, f"DE_{ex_id}_FOLLOW_02", ex["follow_02"]))
        tasks.append((vid, f"DE_{ex_id}_INT_TOPIC", ex["int_topic"]))
        tasks.append((vid, f"DE_{ex_id}_INT_POLITE", ex["int_polite"]))
        tasks.append((vid, f"DE_{ex_id}_INT_DIALOG", ex["int_dialog"]))
        tasks.append((vid, f"DE_{ex_id}_OUTRO_01", ex["outro"]))

    # 2. Persona clips (48 x 5 = 240 clips)
    for p in PERSONAS:
        code = p["code"]
        vid = p["voice_id"]
        for exp_code, _, exp_text in EXPRESSIONS:
            tasks.append((vid, f"DE_{code}_{exp_code}", exp_text))
        for line_code, _, _, line_text in SHARED_PARTNER_LINES:
            tasks.append((vid, f"DE_{code}_{line_code}", line_text))
        for ans_code, (_, _, ans_text) in p["answers"].items():
            tasks.append((vid, f"DE_{code}_{ans_code}", ans_text))

    print(f"Total audio clips to check/generate: {len(tasks)}")

    completed = 0
    errors = []
    with ThreadPoolExecutor(max_workers=3) as pool:
        future_map = {
            pool.submit(
                synthesize_clip,
                api_key,
                vid,
                text,
                CACHE_AUDIO_DIR / f"{audio_id}.mp3",
            ): audio_id
            for vid, audio_id, text in tasks
        }
        for fut in as_completed(future_map):
            aid = future_map[fut]
            completed += 1
            try:
                status = fut.result()
                print(f"[{completed:03d}/{len(tasks):03d}] {status}")
            except Exception as exc:
                print(f"[{completed:03d}/{len(tasks):03d}] ERROR {aid}: {exc}", file=sys.stderr)
                errors.append((aid, str(exc)))

    if errors:
        print(f"\nEncountered {len(errors)} errors during synthesis:", file=sys.stderr)
        for aid, err in errors:
            print(f"  - {aid}: {err}", file=sys.stderr)
        sys.exit(1)

    # Assemble output_s3 and upload_package/telc/b1/
    manifest_exams = []
    for p in PERSONAS:
        code = p["code"]
        speaker = p["speaker"]
        ex = EXAMINERS[p["examiner_idx"]]
        ex_id = ex["id"]
        ex_name = ex["name"]

        # Targets
        sample_folder = OUTPUT_DIR / p["folder"] / "Teil_1"
        sample_folder.mkdir(parents=True, exist_ok=True)

        upload_sample_folder = UPLOAD_ROOT / "speaking" / p["folder"] / "Teil_1"
        upload_sample_folder.mkdir(parents=True, exist_ok=True)

        dialogues = build_persona_dialogues(p, ex)
        script_obj = {
            "voice_profiles": {
                ex_name: ex["voice_id"],
                speaker: p["voice_id"],
            },
            "auto_play_sequence": [
                f"DE_{ex_id}_INTRO_01",
                f"DE_{code}_OPEN_01",
            ],
            "end_audio_id": f"DE_{ex_id}_OUTRO_01",
            "dialogues": dialogues,
            "teil1 dailogues": dialogues,
        }

        # Write script.json to both targets
        for sf in (sample_folder, upload_sample_folder):
            script_path = sf / "script.json"
            script_path.write_text(
                json.dumps(script_obj, ensure_ascii=False, indent=2) + "\n",
                encoding="utf-8",
            )

        # Collect and copy all referenced audio files
        referenced_ids = set()
        for d in dialogues:
            inter = d["interaction"]
            for k in ("question", "answer", "dialogue"):
                if k in inter and inter[k] and "audio_id" in inter[k]:
                    referenced_ids.add(inter[k]["audio_id"])

        for aid in sorted(referenced_ids):
            src = CACHE_AUDIO_DIR / f"{aid}.mp3"
            for sf in (sample_folder, upload_sample_folder):
                dst = sf / f"{aid}.mp3"
                shutil.copy2(src, dst)

        print(
            f"Prepared {p['folder']}/Teil_1: Examiner: {ex_name} ({ex['voice_id']}) | Partner: {speaker} ({p['voice_id']}) | {len(referenced_ids)} MP3s"
        )

        manifest_exams.append(
            {
                "id": p["sample_id"],
                "name": p["sample_name"],
                "level": "B1",
                "teils": [
                    {
                        "index": 1,
                        "name": "Teil 1",
                        "path": f"{p['folder']}/Teil_1/",
                        "script": "script.json",
                    }
                ],
            }
        )

    manifest_obj = {
        "project": "German B1 Exam Audios",
        "exams": manifest_exams,
    }
    for mf in (OUTPUT_DIR / "B1_Exams_manifest.json", UPLOAD_ROOT / "speaking" / "B1_Exams_manifest.json"):
        mf.parent.mkdir(parents=True, exist_ok=True)
        mf.write_text(
            json.dumps(manifest_obj, ensure_ascii=False, indent=2) + "\n",
            encoding="utf-8",
        )
        print(f"Updated {mf}")


if __name__ == "__main__":
    main()
