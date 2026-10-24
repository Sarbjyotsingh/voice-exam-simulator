#!/usr/bin/env python3
"""
Telc B1 Speaking Exam (Teil 3: Gemeinsam etwas planen) — Audio & S3 Manifest Generator

Generates 210 unique ElevenLabs MP3 audio clips across:
- 5 Examiners (Herr Wagner, Frau Fischer, Frau Erdoğan, Herr Richter, Frau Dr. Meyer)
  with official Telc B1 Teil 3 introductions, observer interventions, and the
  final exam sign-off ("Die mündliche Prüfung ist damit beendet...") -> 30 clips.
- 5 Partner Personas (Leyla, Omar, Elena, Mateo, Janusz) across 5 authentic Telc B1
  collaborative planning scenarios (S1–S5):
  - B1_Exam_Sample_01 (Leyla / EX1): S1 — Abschiedsparty für den Deutschkurs organisieren
  - B1_Exam_Sample_02 (Omar / EX2): S2 — Einem Freund beim Renovieren und Umzug helfen
  - B1_Exam_Sample_03 (Elena / EX3): S3 — Gemeinsamer Wochenendausflug / Picknick in der Natur
  - B1_Exam_Sample_04 (Mateo / EX4): S4 — Besuch eines kranken Kollegen / Mitschülers im Krankenhaus
  - B1_Exam_Sample_05 (Janusz / EX5): S5 — Einen Film- und Spieleabend für Freunde veranstalten
- 18 Shared Modular Planning & Negotiation Expressions per Persona (EXP_41–EXP_58) -> 90 clips.
- 5 Shared Structural / Turn-Taking Clips per Persona (T3BACK_Q1, T3BACK_Q2, T3CLAR_Q1, T3SIL_Q1, T3CLOSE_01) -> 25 clips.
- 13 Clean Base Planning Clips per Scenario (T3OPEN_01, TIME_A1, TIME_Q1, TIME_Q2, LOC_Q1, LOC_Q2, LOC_A1,
  FOOD_Q1, FOOD_A1, TASK_Q1, TASK_A1, SUM_A1, SUM_Q1) -> 65 clips.

Populates:
- exam_generator/audio_cache_teil3/
- exam_generator/output_s3/B1_Exam_Sample_01..05/Teil_3/
- exam_generator/upload_package/telc/b1/speaking/B1_Exam_Sample_01..05/Teil_3/
- Updates B1_Exams_manifest.json in both output_s3/ and upload_package/telc/b1/speaking/
  so all exam entries include Teil 1, Teil 2, and Teil 3.

Usage:
  python3 generate_teil3_audio.py --dry-run   # Write script.json + manifests without calling ElevenLabs
  python3 generate_teil3_audio.py             # Synthesize 210 MP3s (cached) and package all 5 scenarios
"""

import argparse
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
CACHE_AUDIO_DIR = BASE_DIR / "audio_cache_teil3"

ELEVENLABS_BASE_URL = "https://api.elevenlabs.io/v1/text-to-speech"
ELEVENLABS_MODEL = "eleven_flash_v2_5"

# ============================================================================
# 1. EXAMINERS (5 Distinct Official Examiners — Same Voice IDs as Teil 1 & 2)
# ============================================================================
EXAMINERS = [
    {
        "id": "EX1",
        "name": "Herr Wagner",
        "voice_id": "Jvf6TAXwMUVTSR20U0f9",
        "t3_intro_01": (
            "Vielen Dank. Wir kommen nun zum dritten und letzten Teil der Prüfung: Gemeinsam etwas planen. "
            "Sie haben das gemeinsame Aufgabenblatt vor sich liegen. Bitte machen Sie Vorschläge, reagieren Sie "
            "aufeinander und einigen Sie sich am Ende auf einen gemeinsamen Plan. Leyla, möchten Sie beginnen?"
        ),
        "t3_int_topic": (
            "Entschuldigung, ich muss kurz unterbrechen: Bitte bleiben Sie bei Ihrer Planungsaufgabe auf dem Aufgabenblatt."
        ),
        "t3_int_dialog": (
            "Kurzer Hinweis: Bitte planen Sie die Aufgabe gemeinsam im Dialog. Machen Sie Vorschläge und fragen Sie "
            "auch Ihre Partnerin nach ihrer Meinung."
        ),
        "t3_int_silence": (
            "Schauen Sie bitte noch einmal kurz auf Ihr Aufgabenblatt: Haben Sie schon alle Punkte besprochen und "
            "die Aufgaben untereinander aufgeteilt?"
        ),
        "t3_int_polite": (
            "Ich bitte Sie um einen höflichen und sachlichen Umgangston während der Prüfung. Bitte fahren Sie fort."
        ),
        "t3_outro_01": (
            "Vielen Dank Ihnen beiden! Damit haben wir auch den dritten Teil geschafft, und Ihre mündliche Prüfung "
            "telc Deutsch B1 ist hiermit beendet. Die Ergebnisse erhalten Sie in circa vier Wochen. Ich wünsche Ihnen "
            "noch einen schönen Tag und alles Gute. Auf Wiedersehen!"
        ),
    },
    {
        "id": "EX2",
        "name": "Frau Fischer",
        "voice_id": "MGG5Irb57ATHvyIeTEYo",
        "t3_intro_01": (
            "Danke schön. Jetzt kommen wir zum dritten und letzten Prüfungsteil: Gemeinsam eine Aufgabe lösen. "
            "Auf Ihrem Aufgabenblatt sehen Sie die Situation und die Punkte, die Sie zusammen planen sollen. "
            "Bitte überlegen Sie gemeinsam, wer was macht. Omar, fangen Sie bitte an."
        ),
        "t3_int_topic": (
            "Darf ich Sie kurz daran erinnern, bei Ihrer gemeinsamen Planungsaufgabe zu bleiben? Bitte planen Sie weiter."
        ),
        "t3_int_dialog": (
            "Bitte denken Sie daran, dass Sie gemeinsam planen sollen. Gehen Sie auf die Vorschläge Ihres Partners "
            "ein und teilen Sie die Aufgaben auf."
        ),
        "t3_int_silence": (
            "Überlegen Sie bitte noch kurz gemeinsam: Haben Sie schon alle Punkte auf dem Blatt geklärt und "
            "festgelegt, wer was übernimmt?"
        ),
        "t3_int_polite": (
            "Bitte achten Sie während der gesamten Prüfung auf eine höfliche und respektvolle Ausdrucksweise."
        ),
        "t3_outro_01": (
            "Herzlichen Dank Ihnen beiden! Das war der dritte Teil, und damit ist Ihre gesamte mündliche Prüfung "
            "beendet. Ihre Ergebnisse bekommen Sie in etwa vier bis sechs Wochen. Wir wünschen Ihnen alles Gute. Tschüss!"
        ),
    },
    {
        "id": "EX3",
        "name": "Frau Erdoğan",
        "voice_id": "zKHQdbB8oaQ7roNTiDTK",
        "t3_intro_01": (
            "Super, vielen Dank. Nun kommen wir zum letzten Teil der Prüfung, Teil drei: Gemeinsam etwas planen. "
            "Sie haben das Aufgabenblatt schon in der Vorbereitungszeit gesehen. Machen Sie bitte Vorschläge, "
            "finden Sie Kompromisse und verteilen Sie die Aufgaben. Elena, möchten Sie starten?"
        ),
        "t3_int_topic": (
            "Einen kleinen Moment bitte: Sie weichen gerade von der Aufgabe ab. Kommen Sie bitte zurück zu Ihrer "
            "gemeinsamen Planung."
        ),
        "t3_int_dialog": (
            "Bitte führen Sie ein gemeinsames Planungsgespräch. Fragen Sie auch Ihre Partnerin, was sie vorschlägt."
        ),
        "t3_int_silence": (
            "Schauen Sie bitte noch einmal auf die Punkte auf Ihrem Blatt: Haben Sie schon geklärt, wer was "
            "mitbringt und wie Sie fahren?"
        ),
        "t3_int_polite": (
            "Ich möchte Sie bitten, höflich und sachlich miteinander umzugehen. Bitte setzen Sie die Planung fort."
        ),
        "t3_outro_01": (
            "Okay, vielen Dank! Das hat sehr gut geklappt. Damit ist Teil drei und auch Ihre gesamte mündliche "
            "Prüfung beendet. Die Notizen lassen Sie bitte hier, und die Ergebnisse erhalten Sie in ungefähr vier "
            "Wochen. Einen schönen Tag noch!"
        ),
    },
    {
        "id": "EX4",
        "name": "Herr Richter",
        "voice_id": "z1EhmmPwF0ENGYE8dBE6",
        "t3_intro_01": (
            "Gut, dann kommen wir jetzt zum dritten und letzten Teil der Prüfung: Gemeinsam etwas planen. "
            "Auf dem Blatt vor Ihnen sehen Sie die Situation. Bitte überlegen Sie zusammen, wann und wie Sie das "
            "machen und wer welche Aufgabe übernimmt. Mateo, fangen Sie bitte an."
        ),
        "t3_int_topic": (
            "Entschuldigung, bitte bleiben Sie bei der Planungsaufgabe auf Ihrem Aufgabenblatt."
        ),
        "t3_int_dialog": (
            "Bitte sprechen Sie nicht nur allein, sondern planen Sie gemeinsam mit Ihrem Partner und reagieren Sie "
            "auf seine Vorschläge."
        ),
        "t3_int_silence": (
            "Prüfen Sie bitte noch kurz auf Ihrem Aufgabenblatt, ob Sie schon alle Punkte und die Aufgabenverteilung "
            "besprochen haben."
        ),
        "t3_int_polite": (
            "Bitte bewahren Sie einen sachlichen und höflichen Ton in der Prüfung."
        ),
        "t3_outro_01": (
            "Besten Dank Ihnen beiden! Damit sind alle drei Teile geschafft und Ihre mündliche Prüfung ist hiermit "
            "offiziell beendet. Ihre Ergebnisse bekommen Sie in vier bis sechs Wochen. Ich wünsche Ihnen alles Gute "
            "und einen schönen Tag!"
        ),
    },
    {
        "id": "EX5",
        "name": "Frau Dr. Meyer",
        "voice_id": "4YKI1ONIhgWEKHyAHYPC",
        "t3_intro_01": (
            "Nun kommen wir zum dritten und letzten Teil der mündlichen Prüfung: Gemeinsam etwas planen. "
            "Sie haben Ihr letztes Aufgabenblatt vor sich. Bitte machen Sie Vorschläge, einigen Sie sich auf die "
            "Details und verteilen Sie die Aufgaben. Janusz, beginnen Sie bitte."
        ),
        "t3_int_topic": (
            "Darf ich Sie kurz unterbrechen? Bitte konzentrieren Sie sich auf Ihre gemeinsame Planungsaufgabe."
        ),
        "t3_int_dialog": (
            "Bitte beziehen Sie Ihren Prüfungspartner aktiv in die Planung mit ein und fragen Sie ihn nach seinen Ideen."
        ),
        "t3_int_silence": (
            "Werfen Sie bitte noch einen Blick auf Ihr Aufgabenblatt: Haben Sie schon alle Punkte geklärt und die "
            "Aufgaben aufgeteilt?"
        ),
        "t3_int_polite": (
            "Ich muss Sie bitten, im Prüfungsgespräch höflich und respektvoll zu bleiben."
        ),
        "t3_outro_01": (
            "Sehr gut gemacht, vielen Dank Ihnen beiden! Damit haben Sie alle Teile der mündlichen Prüfung "
            "erfolgreich abgeschlossen. Die Ergebnisse erhalten Sie in circa vier Wochen. Ich wünsche Ihnen alles "
            "Gute. Auf Wiedersehen!"
        ),
    },
]

# ============================================================================
# 2. CATEGORY A: 18 MODULAR PLANNING & NEGOTIATION EXPRESSIONS (EXP_41–EXP_58)
# ============================================================================
EXPRESSIONS = [
    ("EXP_41", "Expressions (Agreement - Super Idee)", "Das ist eine super Idee!"),
    ("EXP_42", "Expressions (Agreement - So machen wir das)", "Klasse, so machen wir das!"),
    ("EXP_43", "Expressions (Agreement - Passt sehr gut)", "Perfekt, das passt mir sehr gut."),
    ("EXP_44", "Expressions (Agreement - Einverstanden)", "Einverstanden, das klingt nach einem sehr guten Plan."),
    ("EXP_45", "Expressions (Agreement - Sofort dabei)", "Super Vorschlag, da bin ich sofort dabei!"),
    ("EXP_46", "Expressions (Agreement - Abgemacht)", "Alles klar, abgemacht!"),
    ("EXP_47", "Expressions (Doubt - Nicht ganz sicher)", "Hmm, da bin ich mir noch nicht ganz sicher."),
    ("EXP_48", "Expressions (Doubt - Zeitlich knapp)", "Das wird zeitlich vielleicht ein bisschen knapp."),
    ("EXP_49", "Expressions (Doubt - Termin passt nicht)", "Ehrlich gesagt passt mir dieser Termin nicht so gut."),
    (
        "EXP_50",
        "Expressions (Doubt - Schlechtes Wetter)",
        "Die Idee ist gut, aber was machen wir, wenn das Wetter schlecht ist?",
    ),
    ("EXP_51", "Expressions (Counter-Proposal - Anderer Vorschlag)", "Ich hätte da noch einen anderen Vorschlag."),
    (
        "EXP_52",
        "Expressions (Counter-Proposal - Zusammen erledigen)",
        "Wäre es nicht praktischer, wenn wir das zusammen erledigen?",
    ),
    (
        "EXP_53",
        "Expressions (Compromise - Viel praktischer)",
        "Ja, du hast recht, das ist wirklich viel praktischer so.",
    ),
    ("EXP_54", "Expressions (Task Allocation - Kümmere mich)", "Sehr gerne, darum kümmere ich mich!"),
    (
        "EXP_55",
        "Expressions (Task Allocation - Aufgaben teilen)",
        "Abgemacht! Wenn du das übernimmst, kümmere ich mich um den Rest.",
    ),
    (
        "EXP_56",
        "Expressions (Summary Lead-in - Fast alle Punkte)",
        "Super, wir haben jetzt fast alle Punkte geklärt!",
    ),
    (
        "EXP_57",
        "Expressions (Clarification - Verstanden)",
        "Ach so, alles klar, jetzt habe ich dich verstanden.",
    ),
    (
        "EXP_58",
        "Expressions (Politeness - Spaß beim Planen)",
        "Gerne! Es macht richtig Spaß, das mit dir zusammen zu planen.",
    ),
]

# ============================================================================
# 3. CATEGORY B.0: SHARED STRUCTURAL & TURN-TAKING CLIPS FOR TEIL 3
# ============================================================================
SHARED_PARTNER_LINES = [
    ("T3BACK_Q1", "Counter-Question (Planning Feedback 1)", "question", "Was meinst du dazu? Findest du den Vorschlag gut?"),
    ("T3BACK_Q2", "Counter-Question (Planning Feedback 2)", "question", "Hast du vielleicht noch eine andere Idee dazu?"),
    (
        "T3CLAR_Q1",
        "Clarification (Repeat Proposal)",
        "question",
        "Entschuldigung, ich habe deinen Vorschlag akustisch nicht ganz verstanden. Kannst du das bitte noch einmal kurz wiederholen?",
    ),
    (
        "T3SIL_Q1",
        "Silence Recovery",
        "question",
        "Hallo? Bist du noch da? Wollen wir weiter zusammen planen?",
    ),
    (
        "T3CLOSE_01",
        "Closing & Farewell (Teil 3)",
        "dialogue",
        "Super, dann machen wir das genau so! Ich freue mich schon sehr darauf. Bis dann, tschüss!",
    ),
]

# ============================================================================
# 4. CATEGORY B.1–B.5: 5 PERSONAS & PLANNING SCENARIOS (S1–S5)
# ============================================================================
PERSONAS = [
    {
        "sample_id": "B1_S01",
        "sample_name": "B1 Sample Paper 01 (Leyla — Abschiedsparty planen)",
        "folder": "B1_Exam_Sample_01",
        "code": "LEY",
        "scenario_id": "S1",
        "scenario_title": "Abschiedsparty für den Deutschkurs organisieren",
        "exam_hint": (
            "Teil 3: Gemeinsam etwas planen — „Abschiedsparty für den Deutschkurs“\n\n"
            "Situation:\n"
            "Ihr Deutschkurs geht nächste Woche zu Ende. Sie und Ihre Gesprächspartnerin Leyla möchten für "
            "die ganze Gruppe (ca. 15 Personen) und Ihre Lehrerin eine Abschiedsparty organisieren.\n\n"
            "Planen Sie gemeinsam die Party. Überlegen Sie sich:\n"
            "• Wann? (Welcher Tag und welche Uhrzeit passen allen am besten?)\n"
            "• Wo & Wetter? (Im Stadtpark grillen oder ein Raum bei Regen?)\n"
            "• Essen & Getränke? (Internationales Buffet, Getränke, Kosten teilen?)\n"
            "• Geschenk & Aufgabenverteilung? (Kleines Geschenk für die Lehrerin, Musik, Einladung – wer macht was?)"
        ),
        "speaker": "Leyla",
        "full_name": "Leyla Yılmaz",
        "voice_id": "NE7AIW5DoJ7lUosXV2KR",
        "examiner_idx": 0,
        "lines": [
            (
                "T3OPEN_01",
                "Opening & Initial Time Proposal (Wann?)",
                "question",
                "Hallo! Unser Deutschkurs ist ja nächste Woche endlich fertig und wir wollen zusammen eine schöne Abschiedsparty für unsere Gruppe organisieren. Wollen wir zuerst überlegen, wann wir feiern? Ich würde Freitagabend um achtzehn Uhr vorschlagen. Passt dir das, oder hast du einen besseren Termin?",
            ),
            (
                "TIME_A1",
                "Time Counter-Proposal / Compromise (Samstagabend)",
                "answer",
                "Am Samstagabend passt es mir sogar noch besser als am Freitag! Am Freitag sind viele aus dem Kurs nach der Arbeit bestimmt müde, aber am Samstag ab siebzehn Uhr haben alle frei und wir haben viel mehr Zeit zum Feiern.",
            ),
            (
                "TIME_Q1",
                "Time Follow-Up Question (Wann feiern?)",
                "question",
                "An welchem Tag und um wie viel Uhr sollen wir die Abschiedsparty am besten machen? Passt dir Samstagabend?",
            ),
            (
                "TIME_Q2",
                "Time Simpler Re-Ask (Freitag oder Samstag?)",
                "question",
                "Sollen wir die Party lieber am Freitag oder am Samstagabend machen?",
            ),
            (
                "LOC_Q1",
                "Location & Weather Question (Wo feiern?)",
                "question",
                "Und wo wollen wir die Party feiern? Sollen wir bei schönem Wetter im Stadtpark grillen, oder hast du eine Idee, falls es regnet?",
            ),
            (
                "LOC_Q2",
                "Location Simpler Re-Ask (Park oder Raum?)",
                "question",
                "Was meinst du zum Ort: Feiern wir lieber draußen im Park oder drinnen in einem Raum?",
            ),
            (
                "LOC_A1",
                "Location Proposal & Rain Backup (Park + Sprachschule)",
                "answer",
                "Im Stadtpark zu feiern ist bei Sonne super, aber in Deutschland weiß man beim Wetter ja nie. Falls es regnet, könnten wir den großen Gemeinschaftsraum in unserer Sprachschule reservieren. Ich kann morgen im Büro fragen, ob wir den Schlüssel für Samstag bekommen.",
            ),
            (
                "FOOD_Q1",
                "Food & Drinks Question (Essen & Getränke)",
                "question",
                "Wie machen wir das mit dem Essen und den Getränken? Wir sind ja ungefähr fünfzehn Leute aus vielen Ländern – was sollen wir besorgen?",
            ),
            (
                "FOOD_A1",
                "Food & Drinks Proposal (Buffet & Getränke)",
                "answer",
                "Wenn jeder aus dem Kurs eine Spezialität aus seinem Heimatland mitbringt, haben wir ein tolles internationales Buffet! Ich mache gerne türkisches Börek und einen großen Salat. Für Wasser, Saft und Pappteller könnten wir einfach von jedem fünf Euro einsammeln.",
            ),
            (
                "TASK_Q1",
                "Gift & Task Division Question (Geschenk & Aufgaben)",
                "question",
                "Wir brauchen natürlich noch Musik und ein kleines Abschiedsgeschenk für unsere Deutschlehrerin. Was könnten wir ihr schenken, und wie teilen wir die Aufgaben unter uns auf?",
            ),
            (
                "TASK_A1",
                "Gift & Task Division Proposal (Blumen, Karte & Musik)",
                "answer",
                "Ein Blumenstrauß, eine Packung Pralinen und eine schöne Karte mit den Unterschriften von allen Kursteilnehmern sind eine wunderbare Idee! Ich kann am Freitag die Blumen, die Karte und die Getränke kaufen. Schreibst du dann die Einladung in unsere WhatsApp-Gruppe und bringst eine kleine Musikbox mit?",
            ),
            (
                "SUM_A1",
                "Final Plan Summary (Zusammenfassung)",
                "answer",
                "Super, dann fasse ich unseren Plan kurz zusammen: Wir feiern am Samstag ab siebzehn Uhr im Park oder bei Regen im Raum der Sprachschule. Jeder bringt etwas für das Buffet mit, ich besorge die Getränke sowie die Blumen und die Karte für unsere Lehrerin, und du schreibst in die WhatsApp-Gruppe und kümmerst dich um die Musik.",
            ),
            (
                "SUM_Q1",
                "Summary Completeness Check (Alles besprochen?)",
                "question",
                "Haben wir damit an alles gedacht, oder haben wir noch etwas Wichtiges für die Party vergessen?",
            ),
        ],
    },
    {
        "sample_id": "B1_S02",
        "sample_name": "B1 Sample Paper 02 (Omar — Renovieren & Umzug helfen)",
        "folder": "B1_Exam_Sample_02",
        "code": "OMA",
        "scenario_id": "S2",
        "scenario_title": "Einem Freund beim Renovieren und Umzug helfen",
        "exam_hint": (
            "Teil 3: Gemeinsam etwas planen — „Einem Freund beim Renovieren und Umzug helfen“\n\n"
            "Situation:\n"
            "Ihr gemeinsamer Freund Karim zieht in eine neue Wohnung um. Vor dem Einzug müssen noch zwei Zimmer "
            "gestrichen und die Möbel transportiert werden. Sie und Ihr Gesprächspartner Omar möchten ihm helfen.\n\n"
            "Planen Sie gemeinsam die Hilfe. Überlegen Sie sich:\n"
            "• Wann? (An welchem Tag und um wie viel Uhr? Achtung: Ruhezeiten am Sonntag!)\n"
            "• Werkzeuge & Material? (Farbe, Pinsel, Leiter, Bohrmaschine – wer hat was oder wo kaufen?)\n"
            "• Verpflegung für die Helfer? (Frühstück, Getränke und warmes Mittagessen organisieren)\n"
            "• Transport & Aufgabenverteilung? (Auto oder Kleintransporter für die Möbel – wer kümmert sich um was?)"
        ),
        "speaker": "Omar",
        "full_name": "Omar Al-Masri",
        "voice_id": "A2eYGaZmwykOiEVBlQ7h",
        "examiner_idx": 1,
        "lines": [
            (
                "T3OPEN_01",
                "Opening & Initial Time Proposal (Wann?)",
                "question",
                "Hallo! Unser Freund Karim zieht ja bald in seine neue Wohnung und braucht unsere Hilfe beim Streichen und beim Umzug. Wollen wir planen, wann wir ihm helfen? Ich hätte diesen Sonntag ab elf Uhr Zeit. Passt dir Sonntag auch, oder ist ein anderer Tag besser?",
            ),
            (
                "TIME_A1",
                "Time Counter-Proposal / Compromise (Samstagmorgen)",
                "answer",
                "Da hast du recht: Am Sonntag darf man in Deutschland wegen der Ruhezeit keinen Lärm mit der Bohrmaschine machen und die Baumärkte sind geschlossen. Dann lass uns lieber am Samstagmorgen früh um neun Uhr anfangen, damit wir bis zum Abend fertig werden!",
            ),
            (
                "TIME_Q1",
                "Time Follow-Up Question (Wann helfen?)",
                "question",
                "Wann sollen wir Karim am besten helfen? Passt dir Samstagmorgen um neun Uhr?",
            ),
            (
                "TIME_Q2",
                "Time Simpler Re-Ask (Samstag oder Sonntag?)",
                "question",
                "An welchem Tag am Wochenende hast du Zeit zum Renovieren – am Samstag oder am Sonntag?",
            ),
            (
                "LOC_Q1",
                "Tools & Hardware Store Question (Werkzeug & Baumarkt)",
                "question",
                "Wir müssen in der neuen Wohnung zuerst das Wohnzimmer und das Schlafzimmer weiß streichen. Hast du zu Hause Werkzeuge wie Pinsel oder eine Leiter, oder wo sollen wir die Farbe und die Sachen besorgen?",
            ),
            (
                "LOC_Q2",
                "Tools Simpler Re-Ask (Farbe & Werkzeug)",
                "question",
                "Wo sollen wir die Wandfarbe und die Werkzeuge kaufen, und was können wir selbst von zu Hause mitbringen?",
            ),
            (
                "LOC_A1",
                "Tools & Hardware Store Proposal (Leiter + Baumarkt)",
                "answer",
                "Ich habe im Keller noch eine große Leiter, eine Bohrmaschine und Abdeckfolie, die bringe ich auf jeden Fall mit. Die weiße Wandfarbe und die Farbrollen können wir am Freitagabend zusammen im großen Baumarkt kaufen, dann verlieren wir am Samstagmorgen keine Zeit.",
            ),
            (
                "FOOD_Q1",
                "Food for Helpers Question (Verpflegung)",
                "question",
                "Vom Streichen und Möbeltragen bekommen wir mittags bestimmt großen Hunger, und in Karims neuer Wohnung steht ja noch keine Küche. Was sollen wir zu essen und zu trinken für alle Helfer organisieren?",
            ),
            (
                "FOOD_A1",
                "Food for Helpers Proposal (Brötchen, Kaffee & Pizza)",
                "answer",
                "Das ist eine super Lösung! Ich bringe morgens frische Brötchen vom Bäcker, eine Kanne Kaffee und einen Kasten Wasser mit. Und zum Mittagessen bestellen wir uns einfach zwei große Familienpizzen direkt in die neue Wohnung.",
            ),
            (
                "TASK_Q1",
                "Transport & Task Division Question (Transporter & Aufgaben)",
                "question",
                "Am Nachmittag müssen wir noch Karims Sofa, das Bett und die Kartons transportieren. Ich habe leider kein eigenes Auto, aber ich habe einen Führerschein. Wie organisieren wir den Transport, und wer kümmert sich um was?",
            ),
            (
                "TASK_A1",
                "Transport & Task Division Proposal (Kleintransporter leihen)",
                "answer",
                "Perfekt! Mein Cousin hat einen großen Kleintransporter, den kann ich mir für Samstag kostenlos ausleihen und ich fahre ihn auch gern. Wenn du am Freitag die Farbe besorgst und am Samstag die Pizza bestellst, hole ich den Transporter, das Werkzeug und das Frühstück.",
            ),
            (
                "SUM_A1",
                "Final Plan Summary (Zusammenfassung)",
                "answer",
                "Klasse, dann steht unser Plan fest: Wir treffen uns am Samstagmorgen um neun Uhr bei Karims neuer Wohnung. Ich leihe den Transporter von meinem Cousin, bringe Werkzeug, Leiter, Kaffee und Brötchen mit, und du kümmerst dich um die Wandfarbe und bestellst mittags die Pizza für uns.",
            ),
            (
                "SUM_Q1",
                "Summary Completeness Check (Alles besprochen?)",
                "question",
                "Haben wir damit alles Wichtige für die Renovierung und den Umzug besprochen, oder fehlt noch etwas?",
            ),
        ],
    },
    {
        "sample_id": "B1_S03",
        "sample_name": "B1 Sample Paper 03 (Elena — Wochenendausflug & Picknick)",
        "folder": "B1_Exam_Sample_03",
        "code": "ELE",
        "scenario_id": "S3",
        "scenario_title": "Gemeinsamer Wochenendausflug / Picknick in der Natur",
        "exam_hint": (
            "Teil 3: Gemeinsam etwas planen — „Wochenendausflug mit Picknick“\n\n"
            "Situation:\n"
            "Am kommenden Wochenende soll das Wetter sehr schön werden. Sie und Ihre Gesprächspartnerin Elena "
            "möchten zusammen einen Tagesausflug in die Natur mit einem Picknick machen.\n\n"
            "Planen Sie gemeinsam den Ausflug. Überlegen Sie sich:\n"
            "• Wann? (Samstag oder Sonntag? Um wie viel Uhr treffen und losfahren?)\n"
            "• Wohin & Wetter-Alternative? (An den See oder in die Berge? Was machen, falls es doch regnet?)\n"
            "• Was mitbringen? (Essen, Getränke, Picknickdecke, Spiele – Achtung: Supermärkte sind sonntags zu!)\n"
            "• Anreise & Aufgabenverteilung? (Mit dem Auto oder mit dem Zug / Deutschlandticket? Wer besorgt was?)"
        ),
        "speaker": "Elena",
        "full_name": "Elena Kovalenko",
        "voice_id": "gVOibprogMfmHVVyo5r6",
        "examiner_idx": 2,
        "lines": [
            (
                "T3OPEN_01",
                "Opening & Initial Time Proposal (Wann?)",
                "question",
                "Hallo! Am Wochenende soll das Wetter ja endlich warm und sonnig werden, deshalb wollen wir zusammen einen schönen Tagesausflug mit einem Picknick machen. Wollen wir zuerst überlegen, an welchem Tag wir fahren? Mir würde Sonntagmorgen um zehn Uhr sehr gut passen. Wie sieht es bei dir aus?",
            ),
            (
                "TIME_A1",
                "Time Counter-Proposal / Compromise (Sonntag 9:30 Uhr)",
                "answer",
                "Alles klar, wenn wir uns schon um neun Uhr dreißig treffen, haben wir den ganzen Tag vor uns und es ist noch nicht so voll. Und falls dir Samstag lieber ist, kann ich meinen Termin verschieben – mir passen beide Tage am Wochenende!",
            ),
            (
                "TIME_Q1",
                "Time Follow-Up Question (Wann starten?)",
                "question",
                "Wann wollen wir unseren Ausflug starten? Passt dir Sonntagmorgen um halb zehn, oder ist dir Samstag lieber?",
            ),
            (
                "TIME_Q2",
                "Time Simpler Re-Ask (Tag & Uhrzeit)",
                "question",
                "An welchem Tag und um wie viel Uhr sollen wir uns für den Ausflug treffen?",
            ),
            (
                "LOC_Q1",
                "Destination & Weather Question (See oder Berge?)",
                "question",
                "Wohin wollen wir eigentlich fahren? Sollen wir lieber an den großen See fahren oder in den Bergen wandern gehen – und was machen wir, falls es plötzlich regnet?",
            ),
            (
                "LOC_Q2",
                "Destination Simpler Re-Ask (See oder Wanderung?)",
                "question",
                "Was gefällt dir besser für unser Picknick: ein Ausflug an den See oder eine Wanderung in den Bergen?",
            ),
            (
                "LOC_A1",
                "Destination Proposal & Rain Backup (Chiemsee + Seecafé)",
                "answer",
                "Ein Ausflug an den Chiemsee ist eine wunderbare Idee! Dort gibt es eine große grüne Wiese direkt am Wasser zum Picknicken und wir können spazieren gehen. Und falls das Wetter doch schlecht wird, gehen wir einfach in das schöne Seecafé oder besuchen das Schloss auf der Insel.",
            ),
            (
                "FOOD_Q1",
                "Picnic Items Question (Essen & Ausrüstung)",
                "question",
                "Was wollen wir für unser Picknick alles zu essen und zu trinken mitnehmen? Am Sonntag sind die Supermärkte ja geschlossen, deshalb müssen wir schon am Samstag einkaufen.",
            ),
            (
                "FOOD_A1",
                "Picnic Items Proposal (Sandwiches, Obst, Decke & Ball)",
                "answer",
                "Genau, am Samstag kaufen wir alles frisch ein! Ich kann gerne Sandwiches mit Käse, frisches Obst wie Erdbeeren und Äpfel sowie eine große Picknickdecke mitbringen. Außerdem nehme ich einen Volleyball und ein Kartenspiel mit, damit uns nicht langweilig wird.",
            ),
            (
                "TASK_Q1",
                "Transport & Task Division Question (Zug/Auto & Aufgaben)",
                "question",
                "Wie wollen wir eigentlich zum See fahren – mit dem Auto oder lieber gemeinsam mit dem Zug? Und was möchtest du für das Picknick besorgen?",
            ),
            (
                "TASK_A1",
                "Transport & Task Division Proposal (Regionalzug + Deutschlandticket)",
                "answer",
                "Mit dem Regionalzug zu fahren ist super praktisch, weil wir alle das Deutschlandticket haben und am See am Wochenende kaum Parkplätze frei sind. Treffen wir uns einfach um neun Uhr fünfzehn am Hauptbahnhof! Ich mache die Sandwiches und das Obst, und du bringst die Getränke und etwas Süßes mit.",
            ),
            (
                "SUM_A1",
                "Final Plan Summary (Zusammenfassung)",
                "answer",
                "Wunderbar, dann fasse ich kurz zusammen: Wir treffen uns am Sonntag um neun Uhr fünfzehn am Hauptbahnhof und fahren mit dem Zug und dem Deutschlandticket an den See. Ich bringe die Picknickdecke, den Ball, Sandwiches und Obst mit, und du kümmerst dich um die Getränke und die Snacks.",
            ),
            (
                "SUM_Q1",
                "Summary Completeness Check (Alles besprochen?)",
                "question",
                "Haben wir damit alles für unseren Ausflug geplant, oder müssen wir noch an etwas denken?",
            ),
        ],
    },
    {
        "sample_id": "B1_S04",
        "sample_name": "B1 Sample Paper 04 (Mateo — Krankenbesuch im Krankenhaus)",
        "folder": "B1_Exam_Sample_04",
        "code": "MAT",
        "scenario_id": "S4",
        "scenario_title": "Besuch eines kranken Kollegen / Mitschülers im Krankenhaus",
        "exam_hint": (
            "Teil 3: Gemeinsam etwas planen — „Krankenbesuch im Krankenhaus“\n\n"
            "Situation:\n"
            "Ihr gemeinsamer Mitschüler und Freund Lukas hatte beim Fußballspielen einen Unfall, hat sich das "
            "Bein gebrochen und liegt seit gestern im Krankenhaus. Sie und Ihr Gesprächspartner Mateo möchten "
            "ihn besuchen und ihm helfen.\n\n"
            "Planen Sie gemeinsam den Besuch. Überlegen Sie sich:\n"
            "• Wann? (An welchem Tag und zu welcher Uhrzeit? Wann sind die Besuchszeiten im Krankenhaus?)\n"
            "• Wo treffen & Wie hinfahren? (Wo treffen Sie sich vorher und fahren Sie mit Bus, Bahn oder Auto?)\n"
            "• Was mitbringen? (Welches Geschenk oder welche Überraschung bringen Sie Lukas gegen die Langeweile mit?)\n"
            "• Wie noch helfen & Aufgabenverteilung? (Hilfe im Alltag oder für den Deutschkurs – wer erledigt was?)"
        ),
        "speaker": "Mateo",
        "full_name": "Mateo Morales",
        "voice_id": "b8tM9jq3trv3MAW23たん",
        "examiner_idx": 3,
        "lines": [
            (
                "T3OPEN_01",
                "Opening & Initial Time Proposal (Wann besuchen?)",
                "question",
                "Hallo! Hast du schon gehört, was passiert ist? Unser Mitschüler Lukas hat sich beim Fußballspielen das Bein gebrochen und liegt im Krankenhaus. Wir sollten ihn unbedingt zusammen besuchen! Ich könnte morgen Nachmittag direkt um vierzehn Uhr. Hast du da schon Zeit, oder musst du arbeiten?",
            ),
            (
                "TIME_A1",
                "Time Counter-Proposal / Compromise (Morgen 16:30 Uhr)",
                "answer",
                "Ach so, du hast bis sechzehn Uhr Unterricht beziehungsweise Arbeit, das verstehe ich. Im städtischen Krankenhaus ist die Besuchszeit zum Glück bis neunzehn Uhr geöffnet! Dann lass uns doch morgen Nachmittag um sechzehn Uhr dreißig gehen, dann haben wir eine gute Stunde Zeit bei ihm.",
            ),
            (
                "TIME_Q1",
                "Time Follow-Up Question (Wann besuchen?)",
                "question",
                "An welchem Tag und um wie viel Uhr wollen wir Lukas im Krankenhaus besuchen? Passt dir morgen um sechzehn Uhr dreißig?",
            ),
            (
                "TIME_Q2",
                "Time Simpler Re-Ask (Morgen oder Wochenende?)",
                "question",
                "Wann hast du Zeit für den Krankenbesuch – morgen am späten Nachmittag oder lieber am Wochenende?",
            ),
            (
                "LOC_Q1",
                "Meeting Point & Transport Question (Treffpunkt & Anfahrt)",
                "question",
                "Wo sollen wir uns vor dem Besuch treffen, und wie kommen wir am besten zum Krankenhaus – mit dem Bus, mit der Straßenbahn oder mit dem Auto?",
            ),
            (
                "LOC_Q2",
                "Meeting Point Simpler Re-Ask (Wie hinfahren?)",
                "question",
                "Wie wollen wir zum Krankenhaus fahren, und wo treffen wir uns vorher?",
            ),
            (
                "LOC_A1",
                "Meeting Point & Transport Proposal (Bahnhof + Straßenbahn Linie 4)",
                "answer",
                "Beim Krankenhaus ist das Parken extrem teuer, deshalb fahren wir am besten mit der Straßenbahn Linie vier. Die hält direkt vor dem Haupteingang! Lass uns um sechzehn Uhr fünfzehn am Bahnhof vor der Bäckerei treffen und dann zusammen hinfahren.",
            ),
            (
                "FOOD_Q1",
                "Gift & Care Items Question (Geschenk & Überraschung)",
                "question",
                "Wir können natürlich nicht mit leeren Händen ins Krankenhaus kommen. Im Krankenhaus ist es ja oft total langweilig – was sollen wir Lukas als Geschenk oder kleine Überraschung mitbringen?",
            ),
            (
                "FOOD_A1",
                "Gift & Care Items Proposal (Zeitschriften, Obst, Schokolade & Karte)",
                "answer",
                "Ein spannendes Buch oder ein paar Sport-Zeitschriften, frisches Obst und seine Lieblingsschokolade sind eine super Idee! Außerdem können wir eine schöne Genesungskarte kaufen, auf der alle aus unserem Deutschkurs unterschreiben.",
            ),
            (
                "TASK_Q1",
                "Helping Lukas & Task Division Question (Hilfe & Aufgaben)",
                "question",
                "Da Lukas mit seinem Gipsbein bestimmt zwei Wochen nicht in den Kurs kommen und auch nicht selbst einkaufen kann: Wie können wir ihm noch helfen, und wer von uns besorgt was für morgen?",
            ),
            (
                "TASK_A1",
                "Helping Lukas & Task Division Proposal (Kursnotizen & Einkaufen)",
                "answer",
                "Sehr gute Idee! Ich fotografiere jeden Tag unsere Hausaufgaben und Notizen aus dem Deutschkurs und schicke sie ihm per WhatsApp, und ich kaufe heute noch die Karte und die Zeitschriften. Besorgst du dann morgen das Obst und die Schokolade, und wir fragen ihn im Krankenhaus, ob wir am Wochenende für ihn im Supermarkt einkaufen sollen?",
            ),
            (
                "SUM_A1",
                "Final Plan Summary (Zusammenfassung)",
                "answer",
                "Alles klar, dann fasse ich kurz zusammen: Wir treffen uns morgen um sechzehn Uhr fünfzehn am Bahnhof und fahren mit der Straßenbahn zum Krankenhaus. Ich kümmere mich um die Genesungskarte mit den Unterschriften, die Zeitschriften und die Kursnotizen, und du bringst das Obst und die Schokolade mit.",
            ),
            (
                "SUM_Q1",
                "Summary Completeness Check (Alles besprochen?)",
                "question",
                "Haben wir damit an alles für unseren Krankenbesuch gedacht, oder fällt dir noch etwas ein?",
            ),
        ],
    },
    {
        "sample_id": "B1_S05",
        "sample_name": "B1 Sample Paper 05 (Janusz — Film- & Spieleabend veranstalten)",
        "folder": "B1_Exam_Sample_05",
        "code": "JAN",
        "scenario_id": "S5",
        "scenario_title": "Einen Film- und Spieleabend für Freunde veranstalten",
        "exam_hint": (
            "Teil 3: Gemeinsam etwas planen — „Film- und Spieleabend für Freunde“\n\n"
            "Situation:\n"
            "Sie und Ihr Gesprächspartner Janusz möchten am Wochenende einen gemütlichen Film- und Spieleabend "
            "für ein paar gemeinsame Freunde aus Ihrem Deutschkurs veranstalten.\n\n"
            "Planen Sie gemeinsam den Abend. Überlegen Sie sich:\n"
            "• Wann? (Freitagabend oder Samstagabend? Um wie viel Uhr sollen die Gäste kommen?)\n"
            "• Wo / Bei wem? (Bei wem zu Hause findet der Abend statt? Gibt es genug Platz und einen großen Fernseher?)\n"
            "• Welcher Film & welche Spiele? (Komödie oder Actionfilm auf Deutsch? Welche Spiele spielen Sie danach?)\n"
            "• Essen, Getränke & Aufgabenverteilung? (Pizza bestellen oder Snacks machen? Wer kauft was und wer lädt ein?)"
        ),
        "speaker": "Janusz",
        "full_name": "Janusz Kamiński",
        "voice_id": "b0bd425N3mW7jPTkL4K2",
        "examiner_idx": 4,
        "lines": [
            (
                "T3OPEN_01",
                "Opening & Initial Time Proposal (Wann?)",
                "question",
                "Hallo! Wir wollten doch schon lange mal einen gemütlichen Film- und Spieleabend für unsere Freunde aus dem Deutschkurs organisieren. Lass uns gleich überlegen, wann wir das machen! Wie wäre es diesen Freitagabend um neunzehn Uhr, oder passt dir Samstagabend besser?",
            ),
            (
                "TIME_A1",
                "Time Counter-Proposal / Compromise (Samstag 19:30 Uhr)",
                "answer",
                "Da hast du völlig recht: Am Freitagabend sind viele nach der ganzen Woche im Kurs und bei der Arbeit zu müde. Am Samstagabend um neunzehn Uhr dreißig ist es viel entspannter, weil am Sonntag alle ausschlafen können!",
            ),
            (
                "TIME_Q1",
                "Time Follow-Up Question (Welcher Abend?)",
                "question",
                "An welchem Tag und um wie viel Uhr wollen wir den Filmabend machen? Sollen wir Samstag um neunzehn Uhr dreißig festlegen?",
            ),
            (
                "TIME_Q2",
                "Time Simpler Re-Ask (Freitag oder Samstag?)",
                "question",
                "Passt dir für unseren Filmabend lieber der Freitagabend oder der Samstagabend?",
            ),
            (
                "LOC_Q1",
                "Location / Whose Home Question (Bei wem zu Hause?)",
                "question",
                "Und bei wem zu Hause sollen wir den Abend machen? Wir sind ungefähr sechs Personen – hast du genug Platz bei dir, oder wollen wir uns bei mir treffen?",
            ),
            (
                "LOC_Q2",
                "Location Simpler Re-Ask (Bei dir oder bei mir?)",
                "question",
                "Wo wollen wir uns treffen: bei dir in der Wohnung oder bei mir im Haus?",
            ),
            (
                "LOC_A1",
                "Location Proposal (Janusz' Wohnzimmer in Essen)",
                "answer",
                "Wir können das sehr gerne bei mir zu Hause in Essen machen! Wir haben ein großes Wohnzimmer mit einem gemütlichen Sofa, einem großen Fernseher und genug Stühlen für sechs Leute. Außerdem stören wir im Haus keine Nachbarn, wenn wir später noch Spiele spielen und lachen.",
            ),
            (
                "FOOD_Q1",
                "Snacks & Drinks Question (Essen & Getränke)",
                "question",
                "Zu einem guten Filmabend gehören natürlich auch leckeres Essen und Getränke. Wollen wir selbst etwas kochen, oder was schlägst du für Essen und Trinken vor?",
            ),
            (
                "FOOD_A1",
                "Snacks & Drinks Proposal (Familienpizza, Popcorn & Getränke)",
                "answer",
                "Lange in der Küche zu stehen ist an dem Abend zu stressig. Wenn wir beim Italiener zwei große Familienpizzen bestellen – eine mit Gemüse und eine mit Salami – und dazu Chips, Popcorn, Cola und Saft auf den Tisch stellen, ist für jeden etwas dabei!",
            ),
            (
                "TASK_Q1",
                "Movie/Games & Task Division Question (Filmauswahl & Aufgaben)",
                "question",
                "Welchen Film und welche Spiele wollen wir eigentlich auswählen? Und wie teilen wir die Aufgaben auf – wer besorgt die Getränke und die Snacks?",
            ),
            (
                "TASK_A1",
                "Movie/Games & Task Division Proposal (Deutsche Komödie & Spiele)",
                "answer",
                "Eine lustige deutsche Komödie mit deutschen Untertiteln ist perfekt für unsere Gruppe, weil eine Serie zu lange dauert, und danach können wir noch Karten oder Activity spielen! Ich bereite das Wohnzimmer vor, kaufe die Getränke und suche zwei Filme zur Auswahl heraus. Bringst du dann die Chips und das Popcorn mit und gibst den anderen in der Gruppe Bescheid?",
            ),
            (
                "SUM_A1",
                "Final Plan Summary (Zusammenfassung)",
                "answer",
                "Prima, dann fasse ich unseren Plan zusammen: Wir treffen uns am Samstagabend um neunzehn Uhr dreißig bei mir zu Hause. Wir bestellen zusammen Pizza, schauen eine deutsche Komödie und spielen danach Gesellschaftsspiele. Ich besorge die Getränke und bereite alles vor, und du bringst die Snacks mit und lädst unsere Freunde ein.",
            ),
            (
                "SUM_Q1",
                "Summary Completeness Check (Alles besprochen?)",
                "question",
                "Haben wir damit alles für unseren Film- und Spieleabend besprochen, oder fehlt noch eine Kleinigkeit?",
            ),
        ],
    },
]

# Ensure exact voice ID parity with generate_teil1_audio.py and generate_teil2_audio.py
def _verify_persona_voice_ids():
    """Ensure exact voice ID parity with generate_teil1_audio.py and generate_teil2_audio.py."""
    exact_voices = {
        "LEY": "NE7AIW5DoJ7lUosXV2KR",
        "OMA": "A2eYGaZmwykOiEVBlQ7h",
        "ELE": "gVOibprogMfmHVVyo5r6",
        "MAT": "1J0wWp4zPQIvsK7Xwh34",
        "JAN": "K5ZVtkkBnuPY6YqXs70E",
    }
    for p in PERSONAS:
        p["voice_id"] = exact_voices[p["code"]]


_verify_persona_voice_ids()


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


def synthesize_clip(api_key: str, voice_id: str, text: str, out_path: Path) -> str:
    if out_path.exists() and out_path.stat().st_size > 1024:
        return f"CACHED   {out_path.name}"

    url = f"{ELEVENLABS_BASE_URL}/{voice_id}"
    payload = {
        "text": text,
        "model_id": ELEVENLABS_MODEL,
        "language_code": "de",
        "voice_settings": {
            "stability": 0.55,
            "similarity_boost": 0.78,
            "style": 0.15,
            "use_speaker_boost": True,
        },
    }
    data = json.dumps(payload).encode("utf-8")

    for attempt in range(1, 5):
        req = urllib.request.Request(
            url,
            data=data,
            headers={
                "xi-api-key": api_key,
                "Content-Type": "application/json",
                "Accept": "audio/mpeg",
            },
            method="POST",
        )
        try:
            with urllib.request.urlopen(req, timeout=45) as resp:
                audio_bytes = resp.read()
                if len(audio_bytes) < 512:
                    raise RuntimeError(f"Audio response too small ({len(audio_bytes)} bytes)")
                out_path.parent.mkdir(parents=True, exist_ok=True)
                out_path.write_bytes(audio_bytes)
                return f"CREATED  {out_path.name} ({len(audio_bytes)} bytes)"
        except urllib.error.HTTPError as e:
            err_body = e.read().decode("utf-8", errors="replace")
            if e.code == 429 or e.code >= 500:
                wait_s = 2.0 * attempt
                time.sleep(wait_s)
                continue
            raise RuntimeError(f"HTTP {e.code} for {out_path.name}: {err_body}") from e
        except Exception as e:
            if attempt < 4:
                time.sleep(2.0 * attempt)
                continue
            raise RuntimeError(f"Failed {out_path.name}: {e}") from e

    raise RuntimeError(f"Exhausted retries for {out_path.name}")


def build_teil3_dialogues(persona: dict, examiner: dict) -> list:
    code = persona["code"]
    scen = persona["scenario_id"]
    speaker = persona["speaker"]
    ex_id = examiner["id"]
    ex_name = examiner["name"]

    dialogues = []

    def add_entry(topic: str, role: str, spk: str, kind: str, audio_id: str, text: str):
        dialogues.append(
            {
                "topic": topic,
                "speaker": spk,
                "role": role,
                "speaker_role": role,
                "interaction": {
                    kind: {
                        "speaker": spk,
                        "text": text,
                        "german": text,
                        "english": "",
                        "audio_id": audio_id,
                    }
                },
            }
        )

    # 1. Examiner Intro
    add_entry(
        "Introduction (Teil 3)",
        "Examiner",
        ex_name,
        "dialogue",
        f"DE_{ex_id}_T3_INTRO_01",
        examiner["t3_intro_01"],
    )

    # 2. Scenario-specific clean base lines (Opening + 4 Planning Slots + Summary)
    for slot_suffix, topic_label, kind, text in persona["lines"]:
        aid = f"DE_{code}_{scen}_{slot_suffix}"
        add_entry(topic_label, "Participant", speaker, kind, aid, text)

    # 3. Shared structural & turn-taking lines (T3BACK_Q1, T3BACK_Q2, T3CLAR_Q1, T3SIL_Q1, T3CLOSE_01)
    for line_code, topic_label, kind, text in SHARED_PARTNER_LINES:
        aid = f"DE_{code}_{line_code}"
        add_entry(topic_label, "Participant", speaker, kind, aid, text)

    # 4. Shared Modular Planning & Negotiation Expressions (EXP_41–EXP_58)
    for exp_code, topic_label, text in EXPRESSIONS:
        aid = f"DE_{code}_{exp_code}"
        add_entry(topic_label, "Participant", speaker, "dialogue", aid, text)

    # 5. Examiner Interventions & Official Final Exam Sign-Off
    add_entry(
        "Examiner Intervention (Off-Topic)",
        "Examiner",
        ex_name,
        "dialogue",
        f"DE_{ex_id}_T3_INT_TOPIC",
        examiner["t3_int_topic"],
    )
    add_entry(
        "Examiner Intervention (Monologue / No Negotiation)",
        "Examiner",
        ex_name,
        "dialogue",
        f"DE_{ex_id}_T3_INT_DIALOG",
        examiner["t3_int_dialog"],
    )
    add_entry(
        "Examiner Intervention (Skipped Bullet Points / Silence)",
        "Examiner",
        ex_name,
        "dialogue",
        f"DE_{ex_id}_T3_INT_SILENCE",
        examiner["t3_int_silence"],
    )
    add_entry(
        "Examiner Intervention (Politeness)",
        "Examiner",
        ex_name,
        "dialogue",
        f"DE_{ex_id}_T3_INT_POLITE",
        examiner["t3_int_polite"],
    )
    add_entry(
        "Official Conclusion of the Oral Exam (Teil 3 Outro)",
        "Examiner",
        ex_name,
        "dialogue",
        f"DE_{ex_id}_T3_OUTRO_01",
        examiner["t3_outro_01"],
    )

    return dialogues


def update_manifests():
    """
    Update B1_Exams_manifest.json in both output_s3/ and upload_package/telc/b1/speaking/
    so that every exam entry includes Teil 1, Teil 2, and Teil 3.
    """
    for mf in (
        OUTPUT_DIR / "B1_Exams_manifest.json",
        UPLOAD_ROOT / "speaking" / "B1_Exams_manifest.json",
    ):
        if mf.exists():
            data = json.loads(mf.read_text(encoding="utf-8"))
        else:
            data = {
                "project": "German B1 Exam Audios",
                "exams": [
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
                    for p in PERSONAS
                ],
            }

        for exam in data.get("exams", []):
            teils = exam.get("teils", [])
            # Determine the base sample folder (B1_Exam_Sample_01..05) from Teil 1 path
            t1_folder = "B1_Exam_Sample_01"
            for t in teils:
                if t.get("index") == 1 and t.get("path"):
                    t1_folder = t["path"].split("/")[0]
                    break
            teil3_entry = {
                "index": 3,
                "name": "Teil 3",
                "path": f"{t1_folder}/Teil_3/",
                "script": "script.json",
            }
            replaced = False
            for idx, t in enumerate(teils):
                if t.get("index") == 3:
                    teils[idx] = teil3_entry
                    replaced = True
                    break
            if not replaced:
                teils.append(teil3_entry)
            exam["teils"] = sorted(teils, key=lambda x: x.get("index", 0))

        mf.parent.mkdir(parents=True, exist_ok=True)
        mf.write_text(
            json.dumps(data, ensure_ascii=False, indent=2) + "\n",
            encoding="utf-8",
        )
        print(f"Updated manifest: {mf} ({len(data.get('exams', []))} exams with Teil 3)")


def main():
    parser = argparse.ArgumentParser(description="Generate Telc B1 Teil 3 audio and S3 packages.")
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Write script.json and manifests without calling ElevenLabs API.",
    )
    args = parser.parse_args()

    CACHE_AUDIO_DIR.mkdir(parents=True, exist_ok=True)
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)

    tasks = []

    # 1. Examiner clips (6 per examiner x 5 = 30 clips)
    for ex in EXAMINERS:
        ex_id = ex["id"]
        vid = ex["voice_id"]
        tasks.append((vid, f"DE_{ex_id}_T3_INTRO_01", ex["t3_intro_01"]))
        tasks.append((vid, f"DE_{ex_id}_T3_INT_TOPIC", ex["t3_int_topic"]))
        tasks.append((vid, f"DE_{ex_id}_T3_INT_DIALOG", ex["t3_int_dialog"]))
        tasks.append((vid, f"DE_{ex_id}_T3_INT_SILENCE", ex["t3_int_silence"]))
        tasks.append((vid, f"DE_{ex_id}_T3_INT_POLITE", ex["t3_int_polite"]))
        tasks.append((vid, f"DE_{ex_id}_T3_OUTRO_01", ex["t3_outro_01"]))

    # 2. Persona clips ((18 expressions + 5 shared + 13 scenario) x 5 = 180 clips)
    for p in PERSONAS:
        code = p["code"]
        scen = p["scenario_id"]
        vid = p["voice_id"]
        for exp_code, _, exp_text in EXPRESSIONS:
            tasks.append((vid, f"DE_{code}_{exp_code}", exp_text))
        for line_code, _, _, line_text in SHARED_PARTNER_LINES:
            tasks.append((vid, f"DE_{code}_{line_code}", line_text))
        for slot_suffix, _, _, line_text in p["lines"]:
            tasks.append((vid, f"DE_{code}_{scen}_{slot_suffix}", line_text))

    print(f"Total unique Teil 3 audio clips to check/generate: {len(tasks)}")

    if not args.dry_run:
        api_key = load_api_key()
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
                    print(
                        f"[{completed:03d}/{len(tasks):03d}] ERROR {aid}: {exc}",
                        file=sys.stderr,
                    )
                    errors.append((aid, str(exc)))

        if errors:
            print(
                f"\nEncountered {len(errors)} errors during synthesis:",
                file=sys.stderr,
            )
            for aid, err in errors:
                print(f"  - {aid}: {err}", file=sys.stderr)
            sys.exit(1)

    # Assemble output_s3/B1_Exam_Sample_0X/Teil_3/ and upload_package/telc/b1/speaking/B1_Exam_Sample_0X/Teil_3/
    for p in PERSONAS:
        code = p["code"]
        scen = p["scenario_id"]
        speaker = p["speaker"]
        ex = EXAMINERS[p["examiner_idx"]]
        ex_id = ex["id"]
        ex_name = ex["name"]

        sample_folder = OUTPUT_DIR / p["folder"] / "Teil_3"
        sample_folder.mkdir(parents=True, exist_ok=True)

        upload_sample_folder = UPLOAD_ROOT / "speaking" / p["folder"] / "Teil_3"
        upload_sample_folder.mkdir(parents=True, exist_ok=True)

        dialogues = build_teil3_dialogues(p, ex)
        script_obj = {
            "scenario_id": scen,
            "scenario_title": p["scenario_title"],
            "exam_hint": p["exam_hint"],
            "voice_profiles": {
                ex_name: ex["voice_id"],
                speaker: p["voice_id"],
            },
            "auto_play_sequence": [
                f"DE_{ex_id}_T3_INTRO_01",
                f"DE_{code}_{scen}_T3OPEN_01",
            ],
            "end_audio_id": f"DE_{ex_id}_T3_OUTRO_01",
            "dialogues": dialogues,
            "teil3 dialogues": dialogues,
        }

        for sf in (sample_folder, upload_sample_folder):
            script_path = sf / "script.json"
            script_path.write_text(
                json.dumps(script_obj, ensure_ascii=False, indent=2) + "\n",
                encoding="utf-8",
            )

        referenced_ids = set()
        for d in dialogues:
            inter = d["interaction"]
            for k in ("question", "answer", "dialogue"):
                if k in inter and inter[k] and "audio_id" in inter[k]:
                    referenced_ids.add(inter[k]["audio_id"])

        if not args.dry_run:
            for aid in sorted(referenced_ids):
                src = CACHE_AUDIO_DIR / f"{aid}.mp3"
                for sf in (sample_folder, upload_sample_folder):
                    dst = sf / f"{aid}.mp3"
                    shutil.copy2(src, dst)

        print(
            f"Prepared {p['folder']}/Teil_3 ({scen}: {p['scenario_title']}): "
            f"Examiner: {ex_name} ({ex['voice_id']}) | Partner: {speaker} ({p['voice_id']}) | "
            f"{len(referenced_ids)} audio references"
        )

    update_manifests()


if __name__ == "__main__":
    main()
