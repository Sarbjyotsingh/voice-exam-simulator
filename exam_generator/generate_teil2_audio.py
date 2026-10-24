#!/usr/bin/env python3
"""
Telc B1 Speaking Exam - Teil 2 (Gespräch über ein Thema) Audio & Manifest Generator
===================================================================================
Generates all 286 modular MP3 clips and 16 `script.json` packages across:
  - 8 Authentic Telc B1 Topics (T1..T8)
  - 2 Variants per Topic (Variant A: User gets Card A / Partner presents Card B;
    Variant B: User gets Card B / Partner presents Card A) = 16 Practice Scenarios!

Topics & Persona Assignments:
  - T1: Gesunde Ernährung & Kochen vs. Fast Food (Leyla & Herr Wagner)
        - B1_S01 (Variant A): User = Hannes Leivig (Fast Food) | Partner = Bettina Sommer (Frisch kochen)
        - B1_S09 (Variant B): User = Bettina Sommer (Frisch kochen) | Partner = Hannes Leivig (Fast Food)
  - T2: Öffentliche Verkehrsmittel vs. Eigenes Auto (Omar & Frau Fischer)
        - B1_S02 (Variant A): User = Lena Becker (Bus & Bahn) | Partner = Markus Weber (Auto)
        - B1_S10 (Variant B): User = Markus Weber (Auto) | Partner = Lena Becker (Bus & Bahn)
  - T3: Urlaub & Reisen: Allein / Individualreise vs. Gruppenreise (Elena & Frau Erdoğan)
        - B1_S03 (Variant A): User = Sabine Lindner (Allein reisen) | Partner = Thomas Keller (Pauschalreise)
        - B1_S11 (Variant B): User = Thomas Keller (Pauschalreise) | Partner = Sabine Lindner (Allein reisen)
  - T4: Konsum & Shopping: Online-Shopping vs. Geschäft vor Ort (Mateo & Herr Richter)
        - B1_S04 (Variant A): User = Julia Neumann (Online-Shopping) | Partner = Tobias Berg (Geschäft vor Ort)
        - B1_S12 (Variant B): User = Tobias Berg (Geschäft vor Ort) | Partner = Julia Neumann (Online-Shopping)
  - T5: Wohnen: Leben in der Großstadt vs. Leben auf dem Land (Janusz & Frau Dr. Meyer)
        - B1_S05 (Variant A): User = Ilona Schmitz (Großstadt) | Partner = Stefan Baum (Auf dem Land)
        - B1_S13 (Variant B): User = Stefan Baum (Auf dem Land) | Partner = Ilona Schmitz (Großstadt)
  - T6: Soziale Medien & Smartphone im Alltag (Leyla & Herr Wagner)
        - B1_S06 (Variant A): User = Alex Müller (Pro Soziale Medien) | Partner = Ayşe Özel (Kritisch / Zeitverlust)
        - B1_S14 (Variant B): User = Ayşe Özel (Kritisch / Zeitverlust) | Partner = Alex Müller (Pro Soziale Medien)
  - T7: Sport & Bewegung im Alltag (Omar & Frau Fischer)
        - B1_S07 (Variant A): User = Maria Lupo (Täglich Sport & Fitness) | Partner = Luca Giotto (Keine Zeit / Arbeit)
        - B1_S15 (Variant B): User = Luca Giotto (Keine Zeit / Arbeit) | Partner = Maria Lupo (Täglich Sport & Fitness)
  - T8: Rauchen & Gesundheit (Elena & Frau Erdoğan)
        - B1_S08 (Variant A): User = Max Krüger (Raucher seit 20 Jahren) | Partner = Melanie Roth (Striktes Nichtrauchen)
        - B1_S16 (Variant B): User = Melanie Roth (Striktes Nichtrauchen) | Partner = Max Krüger (Raucher seit 20 Jahren)

Usage:
  python3 generate_teil2_audio.py             # Synthesize MP3s via ElevenLabs + write scripts/manifests
  python3 generate_teil2_audio.py --dry-run   # Write scripts/manifests only (skip ElevenLabs API calls)
"""

import argparse
import hashlib
import json
import os
import shutil
import time
import requests
from concurrent.futures import ThreadPoolExecutor, as_completed
from dotenv import load_dotenv

BASE_DIR = os.path.dirname(os.path.abspath(__file__))
load_dotenv(os.path.join(BASE_DIR, "..", ".env"))
load_dotenv(os.path.join(BASE_DIR, ".env"))
ELEVENLABS_API_KEY = os.getenv("ELEVENLABS_API_KEY")

CACHE_DIR = os.path.join(BASE_DIR, "audio_cache_teil2")
OUTPUT_S3_DIR = os.path.join(BASE_DIR, "output_s3")
UPLOAD_PACKAGE_DIR = os.path.join(BASE_DIR, "upload_package", "telc", "b1", "speaking")

MODEL_ID = "eleven_flash_v2_5"
VOICE_SETTINGS = {
    "stability": 0.60,
    "similarity_boost": 0.75,
}

# ==============================================================================
# 1. SHARED MODULAR EXPRESSIONS FOR TEIL 2 (EXP_21 - EXP_38)
# ==============================================================================
SHARED_EXPRESSIONS = [
    ("EXP_21", "Reaktion: Zustimmung", "Da hast du absolut recht."),
    ("EXP_22", "Reaktion: Zustimmung", "Ganz genau, das sehe ich genauso."),
    ("EXP_23", "Reaktion: Zustimmung", "Stimmt, das ist ein sehr guter Punkt."),
    ("EXP_24", "Reaktion: Zustimmung", "Da bin ich ganz deiner Meinung."),
    ("EXP_25", "Reaktion: Zustimmung", "Ja, das kann ich total gut verstehen."),
    ("EXP_26", "Reaktion: Zustimmung", "Das finde ich auch sehr wichtig."),
    ("EXP_27", "Reaktion: Höflicher Widerspruch", "Da bin ich etwas anderer Meinung."),
    ("EXP_28", "Reaktion: Höflicher Widerspruch", "Das sehe ich nicht ganz so."),
    ("EXP_29", "Reaktion: Höflicher Widerspruch", "Ehrlich gesagt ist das bei mir ein bisschen anders."),
    ("EXP_30", "Reaktion: Höflicher Widerspruch", "Ja, vielleicht, aber man muss auch die andere Seite sehen."),
    ("EXP_31", "Reaktion: Höflicher Widerspruch", "Das stimmt zwar, aber für mich persönlich passt das nicht so gut."),
    ("EXP_32", "Reaktion: Interesse", "Ach wirklich? Das ist ja interessant!"),
    ("EXP_33", "Reaktion: Interesse", "Das kann ich mir sehr gut vorstellen."),
    ("EXP_34", "Reaktion: Interesse", "Interessant! So habe ich das noch gar nicht gesehen."),
    ("EXP_35", "Reaktion: Heimatland", "Ach so ist das in deinem Heimatland! Sehr spannend."),
    ("EXP_36", "Reaktion: Zusammenfassung", "Danke für die Zusammenfassung! Das ist eine interessante Meinung."),
    ("EXP_37", "Reaktion: Empathie", "Ja, das kenne ich aus eigener Erfahrung auch sehr gut."),
    ("EXP_38", "Reaktion: Höflichkeit", "Gern geschehen!"),
]

# ==============================================================================
# 2. SHARED STRUCTURAL PARTNER CLIPS FOR TEIL 2
# ==============================================================================
SHARED_PARTNER_STRUCTURAL = {
    "PRES_Q1": "Was steht denn in deinem Text? Kannst du mir deine Meinung auf dem Zettel kurz vorstellen?",
    "PRES_Q2": "Moment mal, bevor wir richtig diskutieren: Was steht eigentlich auf deinem Zettel? Kannst du kurz zusammenfassen, was die Person dort sagt?",
    "T2BACK_Q1": "Und wie siehst du das persönlich?",
    "T2BACK_Q2": "Was hast du für Erfahrungen damit gemacht?",
    "T2CLAR_Q1": "Entschuldigung, das habe ich akustisch nicht ganz verstanden. Kannst du das bitte noch einmal erklären?",
    "T2SIL_Q1": "Bist du noch da? Was meinst du denn zu diesem Thema?",
    "T2CLOSE_01": "Vielen Dank für die spannende Diskussion! Es hat wirklich Spaß gemacht, mit dir über dieses Thema zu sprechen.",
}

# ==============================================================================
# 3. 5 PERSONA PAIRS (REUSED ACROSS THE 8 TOPICS)
# ==============================================================================
PERSONAS = {
    "P1": {
        "teil1_sample_folder": "B1_Exam_Sample_01",
        "partner_short": "Leyla",
        "partner_code": "LEY",
        "partner_voice_id": "NE7AIW5DoJ7lUosXV2KR",
        "examiner_name": "Herr Wagner",
        "examiner_code": "EX1",
        "examiner_voice_id": "Jvf6TAXwMUVTSR20U0f9",
        "examiner_shared": {
            "T2_INTRO_01": "Vielen Dank. Das war Teil 1. Wir kommen nun zu Teil 2: Gespräch über ein Thema. Sie haben in der Vorbereitungszeit beide einen kurzen Text zu einem Thema gelesen. Bitte stellen Sie Ihrem Partner oder Ihrer Partnerin zuerst kurz vor, was die Person in Ihrem Text sagt, und unterhalten Sie sich danach über Ihre eigene Meinung und Ihre Erfahrungen. Leyla, möchten Sie vielleicht beginnen?",
            "T2_INT_TOPIC": "Entschuldigung, ich muss kurz unterbrechen. Bitte bleiben Sie bei Ihrem Prüfungsthema und tauschen Sie Ihre Erfahrungen dazu aus.",
            "T2_INT_POLITE": "Ich bitte Sie um einen höflichen und respektvollen Ton. Das hier ist eine offizielle Sprachprüfung. Bitte fahren Sie sachlich mit der Diskussion fort.",
            "T2_INT_DIALOG": "Denken Sie bitte daran, dass es ein gemeinsames Gespräch sein soll. Stellen Sie auch Ihrer Partnerin Fragen und gehen Sie aufeinander ein.",
            "T2_OUTRO_01": "Vielen Dank. Das war Teil 2. Wir kommen nun zu Teil 3: Gemeinsam eine Aufgabe lösen.",
        },
    },
    "P2": {
        "teil1_sample_folder": "B1_Exam_Sample_02",
        "partner_short": "Omar",
        "partner_code": "OMA",
        "partner_voice_id": "A2eYGaZmwykOiEVBlQ7h",
        "examiner_name": "Frau Fischer",
        "examiner_code": "EX2",
        "examiner_voice_id": "MGG5Irb57ATHvyIeTEYo",
        "examiner_shared": {
            "T2_INTRO_01": "Danke schön. Damit haben wir Teil 1 geschafft und kommen jetzt zu Teil 2: Gespräch über ein Thema. Sie haben beide eine unterschiedliche Meinung zum selben Thema gelesen. Bitte berichten Sie zuerst kurz, was auf Ihrem Aufgabenblatt steht, und diskutieren Sie anschließend gemeinsam darüber. Omar, fangen Sie bitte an.",
            "T2_INT_TOPIC": "Entschuldigung, darf ich Sie kurz daran erinnern, beim Thema Ihrer Textkarten zu bleiben? Bitte diskutieren Sie weiter über das Thema.",
            "T2_INT_POLITE": "Bitte achten Sie auf eine angemessene und respektvolle Ausdrucksweise während der Prüfung. Machen Sie bitte sachlich weiter.",
            "T2_INT_DIALOG": "Bitte führen Sie einen echten Dialog miteinander. Fragen Sie auch Ihren Partner nach seiner Meinung und seinen Erfahrungen.",
            "T2_OUTRO_01": "Herzlichen Dank Ihnen beiden. Das war Teil 2. Dann machen wir jetzt direkt weiter mit Teil 3: Gemeinsam etwas planen.",
        },
    },
    "P3": {
        "teil1_sample_folder": "B1_Exam_Sample_03",
        "partner_short": "Elena",
        "partner_code": "ELE",
        "partner_voice_id": "gVOibprogMfmHVVyo5r6",
        "examiner_name": "Frau Erdoğan",
        "examiner_code": "EX3",
        "examiner_voice_id": "zKHQdbB8oaQ7roNTiDTK",
        "examiner_shared": {
            "T2_INTRO_01": "Super, vielen Dank. Das war schon Teil 1. Kommen wir nun zu Teil 2: Gespräch über ein Thema. Sie haben in der Vorbereitungszeit zwei kurze Texte zu einem Thema bekommen. Stellen Sie sich bitte gegenseitig vor, was Sie gelesen haben, und tauschen Sie dann Ihre persönliche Meinung und Ihre Erfahrungen aus. Elena, möchten Sie starten?",
            "T2_INT_TOPIC": "Einen kleinen Moment bitte — Sie weichen gerade vom Thema ab. Kommen Sie bitte wieder zurück zu Ihrem Prüfungsthema.",
            "T2_INT_POLITE": "Ich möchte Sie bitten, höflich und respektvoll miteinander umzugehen. Bitte setzen Sie das Gespräch sachlich fort.",
            "T2_INT_DIALOG": "Denken Sie bitte daran, dass Sie miteinander diskutieren sollen. Reagieren Sie auf Ihre Partnerin und stellen Sie Fragen.",
            "T2_OUTRO_01": "Vielen Dank, das reicht schon für Teil 2. Sehr schön. Wir kommen jetzt zum dritten und letzten Teil der Prüfung: Gemeinsam etwas planen.",
        },
    },
    "P4": {
        "teil1_sample_folder": "B1_Exam_Sample_04",
        "partner_short": "Mateo",
        "partner_code": "MAT",
        "partner_voice_id": "1J0wWp4zPQIvsK7Xwh34",
        "examiner_name": "Herr Richter",
        "examiner_code": "EX4",
        "examiner_voice_id": "z1EhmmPwF0ENGYE8dBE6",
        "examiner_shared": {
            "T2_INTRO_01": "Gut, vielen Dank für Teil 1. Jetzt machen wir weiter mit Teil 2: Gespräch über ein Thema. Sie haben beide eine Meinung auf Ihrem Aufgabenblatt gelesen. Bitte geben Sie zuerst wieder, was die Person auf Ihrem Blatt sagt, und sprechen Sie danach über Ihre eigenen Erfahrungen und Ansichten. Mateo, fangen Sie bitte an.",
            "T2_INT_TOPIC": "Entschuldigung, bitte bleiben Sie beim Thema Ihrer Aufgabenblätter und diskutieren Sie genau darüber weiter.",
            "T2_INT_POLITE": "Bitte bewahren Sie einen sachlichen und höflichen Ton in der Prüfung. Fahren Sie bitte ordentlich mit dem Gespräch fort.",
            "T2_INT_DIALOG": "Bitte wechseln Sie sich beim Sprechen ab und stellen Sie Ihrem Partner auch Fragen, damit ein richtiges Gespräch entsteht.",
            "T2_OUTRO_01": "Besten Dank Ihnen beiden. Damit ist Teil 2 beendet. Wir kommen nun zum letzten Teil der Prüfung, Teil 3: Gemeinsam eine Aufgabe lösen.",
        },
    },
    "P5": {
        "teil1_sample_folder": "B1_Exam_Sample_05",
        "partner_short": "Janusz",
        "partner_code": "JAN",
        "partner_voice_id": "K5ZVtkkBnuPY6YqXs70E",
        "examiner_name": "Frau Dr. Meyer",
        "examiner_code": "EX5",
        "examiner_voice_id": "4YKI1ONIhgWEKHyAHYPC",
        "examiner_shared": {
            "T2_INTRO_01": "Vielen Dank. Damit schließen wir Teil 1 ab und kommen zum zweiten Teil der Prüfung: Gespräch über ein Thema. Sie haben zwei unterschiedliche Aussagen zum selben Thema vor sich. Bitte berichten Sie einander zuerst kurz, was die Person in Ihrem Text meint, und führen Sie anschließend eine Diskussion über Ihre eigenen Erfahrungen. Janusz, beginnen Sie bitte.",
            "T2_INT_TOPIC": "Darf ich Sie kurz unterbrechen? Sie sind etwas vom Thema abgekommen. Bitte kehren Sie zum Thema Ihrer Textkarten zurück.",
            "T2_INT_POLITE": "Ich muss Sie bitten, höflich und respektvoll zu bleiben. In einer Prüfung erwarten wir einen sachlichen Umgangston.",
            "T2_INT_DIALOG": "Bitte achten Sie darauf, dass Sie gemeinsam diskutieren und auch auf die Argumente Ihres Partners eingehen.",
            "T2_OUTRO_01": "Ich danke Ihnen beiden für dieses Gespräch. Das war der zweite Teil. Wir kommen nun zum dritten Teil der Prüfung: Gemeinsam etwas planen.",
        },
    },
}

# ==============================================================================
# 4. THE 8 CORE TELC B1 TOPICS (EACH WITH CARD A & CARD B FOR BI-DIRECTIONAL PLAY)
# ==============================================================================
TOPICS = [
    {
        "topic_id": "T1",
        "persona_key": "P1",  # Leyla & Herr Wagner
        "title": "Gesunde Ernährung vs. Fast Food",
        "card_a": {
            "person": "Hannes Leivig (32 Jahre, Ingenieur)",
            "stance_label": "Fast Food & Zeitmangel",
            "exam_hint": (
                "Thema: „Gesunde Ernährung & Kochen vs. Fast Food“\n\n"
                "Ihr Textzettel (Hannes Leivig, 32 Jahre, Ingenieur):\n"
                "„Ich arbeite jeden Tag bis 19 Uhr im Büro. Wenn ich abends nach Hause komme, bin ich müde und habe keine Lust mehr, noch lange in der Küche zu stehen. Deshalb esse ich mittags oft etwas Schnelles beim Imbiss und abends mache ich mir eine Tiefkühlpizza. Ich weiß, dass frisches Gemüse gesünder ist, aber mir fehlt einfach die Zeit zum Kochen.“\n\n"
                "Aufgabe:\n"
                "1. Fassen Sie zuerst kurz zusammen, was Hannes Leivig sagt.\n"
                "2. Diskutieren Sie danach mit Ihrer Partnerin: Ihre eigene Meinung, Ihre Essgewohnheiten in Deutschland, wie man sich in Ihrem Heimatland ernährt und welche Vor- und Nachteile Sie sehen."
            ),
            # Used in Variant B when the AI Partner presents Card A (Hannes Leivig)
            "partner_t2open": "Gerne! Mein Text handelt vom Thema Ernährung. Bei mir geht es um Hannes Leivig. Er ist 32 Jahre alt und arbeitet als Ingenieur. Er sagt, dass er jeden Tag bis 19 Uhr arbeitet und abends zu müde zum Kochen ist. Deshalb isst er mittags oft beim Imbiss und abends Tiefkühlpizza, weil ihm einfach die Zeit fehlt. Und was steht in deinem Text?",
            "partner_pres_a1": "In meinem Text sagt Hannes Leivig, ein 32-jähriger Ingenieur, dass er wegen seiner langen Arbeitszeiten fast nie selbst kocht und meistens Fast Food oder Tiefkühlpizza isst.",
            "partner_opin_a1": "Ich kann Hannes ehrlich gesagt gut verstehen. Unter der Woche habe ich wegen der Arbeit auch oft wenig Zeit, aber am Wochenende versuche ich zumindest frisch zu kochen.",
        },
        "card_b": {
            "person": "Bettina Sommer (28 Jahre, Lehrerin)",
            "stance_label": "Frisch kochen & Bio-Gemüse",
            "exam_hint": (
                "Thema: „Gesunde Ernährung & Kochen vs. Fast Food“\n\n"
                "Ihr Textzettel (Bettina Sommer, 28 Jahre, Lehrerin):\n"
                "„Für mich ist gesunde Ernährung extrem wichtig. Ich koche jeden Abend frisch mit viel Gemüse und kaufe fast nur Bio-Produkte auf dem Wochenmarkt. Fast Food oder Fertiggerichte esse ich überhaupt nicht, weil da zu viel Zucker, Fett und Salz drin sind. Kochen dauert zwar länger, aber es entspannt mich nach der Arbeit und hält mich fit.“\n\n"
                "Aufgabe:\n"
                "1. Fassen Sie zuerst kurz zusammen, was Bettina Sommer sagt.\n"
                "2. Diskutieren Sie danach mit Ihrer Partnerin: Ihre eigene Meinung, Ihre Essgewohnheiten in Deutschland, wie man sich in Ihrem Heimatland ernährt und welche Vor- und Nachteile Sie sehen."
            ),
            # Used in Variant A when the AI Partner presents Card B (Bettina Sommer)
            "partner_t2open": "Gerne! Mein Text handelt vom Thema gesunde Ernährung. Bei mir geht es um Bettina Sommer. Sie ist 28 Jahre alt und Lehrerin von Beruf. Sie erzählt, dass sie jeden Abend frisch mit viel Bio-Gemüse kocht und überhaupt kein Fast Food isst, weil Kochen sie entspannt und gesund hält. Und was steht in deinem Text?",
            "partner_pres_a1": "In meinem Text sagt Bettina Sommer, eine 28-jährige Lehrerin, dass sie jeden Tag frisch mit Bio-Gemüse kocht und Fertiggerichte komplett ablehnt.",
            "partner_opin_a1": "Ich persönlich gebe Bettina recht. Ich finde gesunde Ernährung sehr wichtig, weil man sich damit im Alltag einfach fitter und konzentrierter fühlt.",
        },
        "shared_discussion": {
            "OPIN_Q1": "Wie siehst du das? Achtest du auf gesunde Ernährung oder isst du auch oft schnelles Essen?",
            "OPIN_Q2": "Ist gesundes Essen für dich wichtig oder fehlt dir im Alltag oft die Zeit dafür?",
            "HAB_A1": "Hier in Deutschland koche ich meistens abends selbst, oft türkische oder mediterrane Gerichte mit Reis und frischem Gemüse. Nur wenn ich viel Stress habe, hole ich mir mal einen Döner.",
            "HAB_Q1": "Wie machst du das hier in Deutschland? Kochst du meistens selbst zu Hause oder gehst du oft essen?",
            "HAB_Q2": "Was isst du denn normalerweise unter der Woche, wenn du arbeitest oder Deutsch lernst?",
            "HOME_A1": "In meinem Heimatland, der Türkei, kocht man fast jeden Tag frisch für die ganze Familie. Tiefkühlpizza oder Fertiggerichte im Supermarkt sind dort viel seltener als in Deutschland.",
            "HOME_Q1": "Und wie ist das in deinem Heimatland? Kochen die Menschen dort jeden Tag frisch oder gibt es auch viel Fast Food?",
            "PRO_A1": "Der große Vorteil beim Selberkochen ist natürlich, dass es gesünder und günstiger ist. Der Nachteil ist aber, dass Einkaufen und Kochen jeden Tag viel Zeit kosten.",
            "PRO_Q1": "Was sind deiner Meinung nach die größten Vor- und Nachteile von Fast Food im Vergleich zum Selberkochen?",
        },
        "examiner_followup": {
            "T2_FOLLOW_01": "Darf ich noch kurz etwas fragen: Glauben Sie, dass gesunde Lebensmittel im Supermarkt heute zu teuer sind?",
            "T2_FOLLOW_02": "Noch eine kurze Frage an Sie: Sollte man schon in der Schule lernen, wie man gesund kocht?",
        },
    },
    {
        "topic_id": "T2",
        "persona_key": "P2",  # Omar & Frau Fischer
        "title": "Öffentliche Verkehrsmittel vs. Eigenes Auto",
        "card_a": {
            "person": "Lena Becker (25 Jahre, Studentin)",
            "stance_label": "Pro Bus & Bahn (Deutschlandticket)",
            "exam_hint": (
                "Thema: „Öffentliche Verkehrsmittel (Bus & Bahn) vs. Eigenes Auto“\n\n"
                "Ihr Textzettel (Lena Becker, 25 Jahre, Studentin):\n"
                "„Ich habe kein eigenes Auto und brauche auch keins. Mit meinem Deutschlandticket fahre ich jeden Tag bequem mit der Straßenbahn und dem Zug zur Universität. Im Zug kann ich lesen oder Musik hören und muss keinen Parkplatz suchen. Außerdem sind Benzin und Versicherung viel zu teuer und Autofahren schadet der Umwelt.“\n\n"
                "Aufgabe:\n"
                "1. Fassen Sie zuerst kurz zusammen, was Lena Becker sagt.\n"
                "2. Diskutieren Sie danach mit Ihrem Partner: Ihre eigene Meinung, wie Sie in Deutschland unterwegs sind, wie der Verkehr in Ihrem Heimatland ist und welche Vor- und Nachteile Bus, Bahn und Auto haben."
            ),
            "partner_t2open": "Ja, gerne! In meinem Text geht es um das Thema Mobilität, und zwar um Lena Becker. Sie ist 25 Jahre alt und Studentin. Sie sagt, dass sie kein eigenes Auto braucht und mit dem Deutschlandticket jeden Tag bequem Bus und Bahn fährt, weil man im Zug lesen kann und das Auto zu teuer und schlecht für die Umwelt ist. Und was sagt die Person auf deinem Blatt?",
            "partner_pres_a1": "Auf meinem Blatt berichtet Lena Becker, eine 25-jährige Studentin, dass sie lieber mit dem Deutschlandticket Bus und Bahn fährt, weil ein Auto zu teuer und umweltschädlich ist.",
            "partner_opin_a1": "Ich finde Lenas Einstellung super. In einer großen Stadt mit guter Verbindung braucht man wirklich kein eigenes Auto und spart mit dem Deutschlandticket sehr viel Geld.",
        },
        "card_b": {
            "person": "Markus Weber (42 Jahre, Handwerker)",
            "stance_label": "Pro eigenes Auto",
            "exam_hint": (
                "Thema: „Öffentliche Verkehrsmittel (Bus & Bahn) vs. Eigenes Auto“\n\n"
                "Ihr Textzettel (Markus Weber, 42 Jahre, Handwerker):\n"
                "„Ohne mein Auto geht bei mir gar nichts. Ich wohne in einem kleinen Ort außerhalb der Stadt und muss jeden Morgen früh zur Arbeit fahren. Der Bus fährt bei uns nur einmal pro Stunde und die Züge haben ständig Verspätung. Mit dem Auto bin ich flexibel, unabhängig vom Wetter und doppelt so schnell am Ziel.“\n\n"
                "Aufgabe:\n"
                "1. Fassen Sie zuerst kurz zusammen, was Markus Weber sagt.\n"
                "2. Diskutieren Sie danach mit Ihrem Partner: Ihre eigene Meinung, wie Sie in Deutschland unterwegs sind, wie der Verkehr in Ihrem Heimatland ist und welche Vor- und Nachteile Bus, Bahn und Auto haben."
            ),
            "partner_t2open": "Ja, gerne! In meinem Text geht es um das Thema Verkehrsmittel, und zwar um Markus Weber. Er ist 42 Jahre alt und arbeitet als Handwerker. Er wohnt außerhalb der Stadt und sagt, dass er ohne sein Auto nicht zur Arbeit kommt, weil die Busse selten fahren und Züge oft Verspätung haben. Mit dem Auto ist er viel unabhängiger. Und was sagt die Person auf deinem Blatt?",
            "partner_pres_a1": "Auf meinem Blatt berichtet Markus Weber, ein 42-jähriger Handwerker, dass er unbedingt sein Auto braucht, weil Bus und Bahn auf dem Land zu selten und unpünktlich sind.",
            "partner_opin_a1": "Ich kann beide Seiten verstehen, aber für mich persönlich sind öffentliche Verkehrsmittel in der Stadt praktischer, auch wenn mich die Verspätungen bei der Bahn manchmal ärgern.",
        },
        "shared_discussion": {
            "OPIN_Q1": "Wie siehst du das? Fährst du lieber mit dem Auto oder mit Bus und Bahn?",
            "OPIN_Q2": "Was ist für dich im Alltag besser: ein eigenes Auto oder öffentliche Verkehrsmittel?",
            "HAB_A1": "Hier in Deutschland habe ich das Deutschlandticket und fahre jeden Tag mit der U-Bahn und dem Bus zur Arbeit. Ein eigenes Auto ist mir in München wegen der Parkplätze einfach zu teuer.",
            "HAB_Q1": "Und wie bist du hier in Deutschland unterwegs? Nutzt du auch das Deutschlandticket oder hast du ein Auto?",
            "HAB_Q2": "Womit fährst du hier normalerweise zum Einkaufen oder zum Deutschkurs?",
            "HOME_A1": "In meinem Heimatland Syrien fahren die meisten Menschen mit dem Auto oder mit kleinen Sammeltaxis, weil es dort leider kein so gutes Zugnetz wie in Deutschland gibt.",
            "HOME_Q1": "Wie ist die Situation in deinem Heimatland? Gibt es dort gute Busse und Bahnen oder braucht man ein Auto?",
            "PRO_A1": "Der größte Vorteil von Bus und Bahn ist, dass es günstig und gut für das Klima ist. Der größte Nachteil sind aber die häufigen Verspätungen und vollen Züge im Berufsverkehr.",
            "PRO_Q1": "Welche Vor- und Nachteile haben das Auto und die öffentlichen Verkehrsmittel für dich?",
        },
        "examiner_followup": {
            "T2_FOLLOW_01": "Ich hätte noch eine kurze Frage an Sie: Sollten öffentliche Verkehrsmittel in Zukunft komplett kostenlos sein?",
            "T2_FOLLOW_02": "Noch eine Frage zum Abschluss: Glauben Sie, dass es in den Innenstädten ein Fahrverbot für Autos geben sollte?",
        },
    },
    {
        "topic_id": "T3",
        "persona_key": "P3",  # Elena & Frau Erdoğan
        "title": "Urlaub & Reisen: Individualreise vs. Gruppenreise",
        "card_a": {
            "person": "Sabine Lindner (29 Jahre, Grafikdesignerin)",
            "stance_label": "Allein reisen / Individualurlaub",
            "exam_hint": (
                "Thema: „Urlaub & Reisen: Allein reisen vs. Gruppenreise“\n\n"
                "Ihr Textzettel (Sabine Lindner, 29 Jahre, Grafikdesignerin):\n"
                "„Im Urlaub möchte ich frei und unabhängig sein. Deshalb reise ich am liebsten allein mit dem Rucksack und buche nur den Flug vorab. Vor Ort entscheide ich jeden Tag spontan, wohin ich fahre und wo ich übernachte. Bei einer Gruppenreise muss man immer auf andere warten, aber allein lernt man viel schneller Einheimische kennen.“\n\n"
                "Aufgabe:\n"
                "1. Fassen Sie zuerst kurz zusammen, was Sabine Lindner sagt.\n"
                "2. Diskutieren Sie danach mit Ihrer Partnerin: Ihre eigene Meinung zum Thema Reisen, wie Sie selbst Urlaub machen, wie man in Ihrem Heimatland reist und welche Vor- und Nachteile beide Reisearten haben."
            ),
            "partner_t2open": "Natürlich! In meinem Text geht es um Sabine Lindner. Sie ist 29 Jahre alt und Grafikdesignerin von Beruf. Sie reist am liebsten allein mit dem Rucksack und plant fast nichts im Voraus, weil sie frei und spontan sein möchte und so schneller Einheimische kennenlernt. Und welche Meinung steht auf deinem Zettel?",
            "partner_pres_a1": "In meinem Text erzählt Sabine Lindner, eine 29-jährige Grafikdesignerin, dass sie im Urlaub am liebsten allein und spontan mit dem Rucksack reist.",
            "partner_opin_a1": "Ich finde Sabines Mut toll! Ich mag es auch, im Urlaub flexibel zu sein, allerdings reise ich nicht so gern ganz allein, sondern am liebsten zu zweit oder mit guten Freunden.",
        },
        "card_b": {
            "person": "Thomas Keller (47 Jahre, Bankangestellter)",
            "stance_label": "Organisierte Pauschal- & Gruppenreise",
            "exam_hint": (
                "Thema: „Urlaub & Reisen: Allein reisen vs. Gruppenreise“\n\n"
                "Ihr Textzettel (Thomas Keller, 47 Jahre, Bankangestellter):\n"
                "„Ich habe im Beruf das ganze Jahr über genug Stress, deshalb buche ich im Urlaub immer eine organisierte Pauschalreise mit einer Gruppe. Das Hotel, der Bus und die Ausflüge mit dem Reiseleiter sind schon komplett geplant. Ich muss mich um nichts kümmern, fühle mich im Ausland sicher und habe nette Gesellschaft.“\n\n"
                "Aufgabe:\n"
                "1. Fassen Sie zuerst kurz zusammen, was Thomas Keller sagt.\n"
                "2. Diskutieren Sie danach mit Ihrer Partnerin: Ihre eigene Meinung zum Thema Reisen, wie Sie selbst Urlaub machen, wie man in Ihrem Heimatland reist und welche Vor- und Nachteile beide Reisearten haben."
            ),
            "partner_t2open": "Natürlich! In meinem Text geht es um Thomas Keller. Er ist 47 Jahre alt und Bankangestellter von Beruf. Er bucht im Urlaub immer eine organisierte Gruppenreise mit Hotel und Reiseleiter, weil er sich nach der Arbeit um nichts kümmern möchte und sich in der Gruppe sicherer fühlt. Und welche Meinung steht auf deinem Zettel?",
            "partner_pres_a1": "In meinem Text erzählt Thomas Keller, ein 47-jähriger Bankangestellter, dass er organisierte Gruppenreisen bevorzugt, weil dort alles geplant und sicher ist.",
            "partner_opin_a1": "Für mich persönlich wäre eine große Gruppenreise wie bei Thomas nichts. Ich reise lieber individuell mit meinem Mann oder mit Freunden, damit wir selbst entscheiden können, was wir besichtigen.",
        },
        "shared_discussion": {
            "OPIN_Q1": "Wie ist deine Meinung dazu? Reist du lieber individuell und unabhängig oder in einer organisierten Gruppe?",
            "OPIN_Q2": "Was gefällt dir im Urlaub besser: alles selbst planen oder eine fertige Reise buchen?",
            "HAB_A1": "Seit ich in Deutschland lebe, mache ich oft Städtetrips mit dem Zug in Europa oder fliege im Sommer ans Meer. Ich buche das Hotel und die Tickets immer selbst im Internet.",
            "HAB_Q1": "Wie machst du normalerweise Urlaub? Wohin bist du bei deinem letzten Urlaub gereist?",
            "HAB_Q2": "Mit wem verreist du am liebsten, wenn du Urlaub hast?",
            "HOME_A1": "In meinem Heimatland Rumänien fahren viele Familien im Sommer gemeinsam ans Schwarze Meer oder in die Berge. Ältere Menschen buchen dort aber auch sehr gern Busreisen in einer Gruppe.",
            "HOME_Q1": "Und wie machen die Menschen in deinem Heimatland am liebsten Urlaub? Reisen sie eher mit der Familie oder in Reisegruppen?",
            "PRO_A1": "Bei einer Gruppenreise ist der Vorteil, dass man keinen Stress mit der Organisation hat. Dafür hat man bei einer Individualreise viel mehr Freiheit und kann länger bleiben, wo es schön ist.",
            "PRO_Q1": "Was sind für dich die wichtigsten Vor- und Nachteile von Gruppenreisen im Vergleich zum individuellen Reisen?",
        },
        "examiner_followup": {
            "T2_FOLLOW_01": "Darf ich kurz nachfragen: Ist ein Urlaub im eigenen Land für Sie genauso erholsam wie eine weite Flugreise?",
            "T2_FOLLOW_02": "Noch eine kurze Frage: Wie wichtig sind Ihnen Fremdsprachenkenntnisse, wenn Sie in ein anderes Land reisen?",
        },
    },
    {
        "topic_id": "T4",
        "persona_key": "P4",  # Mateo & Herr Richter
        "title": "Einkaufen: Online-Shopping vs. Geschäft vor Ort",
        "card_a": {
            "person": "Julia Neumann (34 Jahre, Buchhalterin)",
            "stance_label": "Pro Online-Shopping",
            "exam_hint": (
                "Thema: „Einkaufen im Internet (Online-Shopping) vs. Einkaufen im Geschäft“\n\n"
                "Ihr Textzettel (Julia Neumann, 34 Jahre, Buchhalterin):\n"
                "„Ich kaufe fast alles im Internet — egal ob Kleidung, Schuhe oder Elektronik. Im Online-Shop habe ich eine riesige Auswahl, kann die Preise sofort vergleichen und auch abends um 22 Uhr bequem vom Sofa aus bestellen. Die Pakete werden direkt an meine Haustür geliefert und ich spare mir volle Geschäfte und lange Schlangen an der Kasse.“\n\n"
                "Aufgabe:\n"
                "1. Fassen Sie zuerst kurz zusammen, was Julia Neumann sagt.\n"
                "2. Diskutieren Sie danach mit Ihrem Partner: Ihre eigene Meinung zum Thema Einkaufen, wie Sie in Deutschland einkaufen, wie das in Ihrem Heimatland ist und welche Vor- und Nachteile Online-Shopping hat."
            ),
            "partner_t2open": "Klar, ich fange gerne an! Auf meinem Blatt geht es um Julia Neumann, sie ist 34 Jahre alt und arbeitet als Buchhalterin. Sie kauft fast alles im Internet, weil sie dort rund um die Uhr eine riesige Auswahl hat, Preise leicht vergleichen kann und alles bequem nach Hause geliefert bekommt. Und was steht auf deinem Aufgabenblatt?",
            "partner_pres_a1": "In meinem Text sagt Julia Neumann, eine 34-jährige Buchhalterin, dass sie fast alles online bestellt, weil die Auswahl größer ist und sie Zeit spart.",
            "partner_opin_a1": "Ich verstehe Julia gut, denn Technik oder Bücher bestelle ich auch gern online. Aber bei Kleidung und Schuhen gehe ich trotzdem lieber in ein echtes Geschäft.",
        },
        "card_b": {
            "person": "Tobias Berg (40 Jahre, Buchhändler)",
            "stance_label": "Pro Einkaufen im Geschäft vor Ort",
            "exam_hint": (
                "Thema: „Einkaufen im Internet (Online-Shopping) vs. Einkaufen im Geschäft“\n\n"
                "Ihr Textzettel (Tobias Berg, 40 Jahre, Buchhändler):\n"
                "„Ich kaufe viel lieber in den Geschäften in unserer Innenstadt ein. Dort kann ich die Sachen direkt anfassen, Kleidung im Laden anprobieren und bekomme eine persönliche Beratung. Beim Online-Shopping passen die Größen oft nicht und man muss ständig Pakete zurückschicken. Außerdem sterben unsere Innenstädte aus, wenn alle nur noch im Internet bestellen.“\n\n"
                "Aufgabe:\n"
                "1. Fassen Sie zuerst kurz zusammen, was Tobias Berg sagt.\n"
                "2. Diskutieren Sie danach mit Ihrem Partner: Ihre eigene Meinung zum Thema Einkaufen, wie Sie in Deutschland einkaufen, wie das in Ihrem Heimatland ist und welche Vor- und Nachteile Online-Shopping hat."
            ),
            "partner_t2open": "Klar, ich fange gerne an! Auf meinem Blatt geht es um Tobias Berg, er ist 40 Jahre alt und arbeitet als Buchhändler. Er kauft viel lieber in den Geschäften in der Innenstadt ein, weil man dort Kleidung direkt anprobieren kann, persönlich beraten wird und nicht ständig Pakete zurückschicken muss. Und was steht auf deinem Aufgabenblatt?",
            "partner_pres_a1": "In meinem Text sagt Tobias Berg, ein 40-jähriger Buchhändler, dass er lieber im Geschäft vor Ort einkauft, weil er persönliche Beratung schätzt und die Innenstädte unterstützen möchte.",
            "partner_opin_a1": "Ich finde, beide haben teilweise recht. Bei Kleidung gehe ich wie Tobias lieber ins Geschäft zum Anprobieren, aber Elektronik oder Bücher bestelle ich oft im Internet.",
        },
        "shared_discussion": {
            "OPIN_Q1": "Wie siehst du das? Kaufst du lieber im Internet ein oder gehst du lieber in ein Geschäft?",
            "OPIN_Q2": "Was gefällt dir persönlich besser: Online-Shopping oder Einkaufen in der Stadt?",
            "HAB_A1": "Hier in Deutschland kaufe ich Lebensmittel und Kleidung meistens im Geschäft in der Stadt. Aber wenn ich ein bestimmtes Kabel oder ein Buch suche, bestelle ich es oft online.",
            "HAB_Q1": "Was kaufst du hier in Deutschland meistens online und wofür gehst du lieber ins Geschäft?",
            "HAB_Q2": "Wie oft bestellst du ungefähr Sachen im Internet?",
            "HOME_A1": "In meinem Heimatland Italien gehen die Menschen immer noch sehr gern in kleine Läden und auf den Markt, weil Einkaufen dort auch ein soziales Erlebnis ist. Aber bei jungen Leuten wächst Online-Shopping natürlich auch stark.",
            "HOME_Q1": "Und wie ist das in deinem Heimatland? Bestellen die Menschen dort auch so viel im Internet oder kaufen sie eher im Laden?",
            "PRO_A1": "Der Vorteil beim Online-Shopping ist, dass es rund um die Uhr geht und oft günstiger ist. Der Nachteil ist aber der viele Verpackungsmüll und dass kleine Läden in der Stadt schließen müssen.",
            "PRO_Q1": "Welche Vor- und Nachteile siehst du beim Online-Shopping im Vergleich zum Einkaufen im Geschäft?",
        },
        "examiner_followup": {
            "T2_FOLLOW_01": "Ich hätte noch eine kurze Frage: Sollte das Zurückschicken von Paketen beim Online-Shopping Geld kosten, um die Umwelt zu schützen?",
            "T2_FOLLOW_02": "Noch eine kurze Frage an Sie: Bezahlen Sie beim Einkaufen lieber bar oder mit Karte und Smartphone?",
        },
    },
    {
        "topic_id": "T5",
        "persona_key": "P5",  # Janusz & Frau Dr. Meyer
        "title": "Wohnen: Leben in der Großstadt vs. Leben auf dem Land",
        "card_a": {
            "person": "Ilona Schmitz (31 Jahre, Architektin)",
            "stance_label": "Pro Leben in der Großstadt",
            "exam_hint": (
                "Thema: „Wohnen: Leben in der Großstadt vs. Leben auf dem Land“\n\n"
                "Ihr Textzettel (Ilona Schmitz, 31 Jahre, Architektin):\n"
                "„Ich lebe mitten in einer Großstadt und möchte niemals aufs Land ziehen. In der Stadt habe ich alles direkt vor der Haustür: Supermärkte, Ärzte, Kinos, internationale Restaurants und gute Arbeitsmöglichkeiten. Man braucht kein Auto und es ist immer etwas los. Die Mieten sind zwar hoch, aber auf dem Land wäre es mir viel zu langweilig.“\n\n"
                "Aufgabe:\n"
                "1. Fassen Sie zuerst kurz zusammen, was Ilona Schmitz sagt.\n"
                "2. Diskutieren Sie danach mit Ihrem Partner: Ihre eigene Meinung zum Thema Wohnen, wie Sie aktuell in Deutschland wohnen, wie die Wohnsituation in Ihrem Heimatland ist und welche Vor- und Nachteile Stadt und Land haben."
            ),
            "partner_t2open": "Gerne, dann fange ich an! In meinem Text spricht Ilona Schmitz. Sie ist 31 Jahre alt und Architektin von Beruf. Sie lebt mitten in einer Großstadt und liebt das Stadtleben, weil Supermärkte, Ärzte, Kultur und gute Jobs direkt vor der Haustür sind und man kein Auto braucht. Auf dem Land wäre es ihr viel zu langweilig. Und was sagt die Person in deinem Text?",
            "partner_pres_a1": "Auf meinem Aufgabenblatt erzählt Ilona Schmitz, eine 31-jährige Architektin, dass sie das Leben in der Großstadt liebt, weil dort alle Geschäfte, Ärzte und Kulturangebote direkt in der Nähe sind.",
            "partner_opin_a1": "Ich kann Ilona gut verstehen, wenn man jung und ledig ist. Aber weil ich selbst zwei Kinder habe, wohne ich lieber etwas ruhiger am Stadtrand im Grünen.",
        },
        "card_b": {
            "person": "Stefan Baum (45 Jahre, IT-Spezialist)",
            "stance_label": "Pro Leben auf dem Land",
            "exam_hint": (
                "Thema: „Wohnen: Leben in der Großstadt vs. Leben auf dem Land“\n\n"
                "Ihr Textzettel (Stefan Baum, 45 Jahre, IT-Spezialist):\n"
                "„Vor drei Jahren bin ich mit meiner Familie aus der Großstadt in ein kleines Dorf auf dem Land gezogen. Hier haben wir ein Haus mit Garten, saubere Luft und endlich Ruhe. Die Kinder können draußen sicher spielen und die Nachbarn kennen und helfen sich noch. Der Lärm, der Stress und die teuren Wohnungen in der Stadt fehlen mir überhaupt nicht.“\n\n"
                "Aufgabe:\n"
                "1. Fassen Sie zuerst kurz zusammen, was Stefan Baum sagt.\n"
                "2. Diskutieren Sie danach mit Ihrem Partner: Ihre eigene Meinung zum Thema Wohnen, wie Sie aktuell in Deutschland wohnen, wie die Wohnsituation in Ihrem Heimatland ist und welche Vor- und Nachteile Stadt und Land haben."
            ),
            "partner_t2open": "Gerne, dann fange ich an! In meinem Text spricht Stefan Baum. Er ist 45 Jahre alt und IT-Spezialist von Beruf. Er ist mit seiner Familie aus der Stadt in ein kleines Dorf aufs Land gezogen, weil sie dort ein Haus mit Garten, saubere Luft und viel Ruhe für die Kinder haben. Und was sagt die Person in deinem Text?",
            "partner_pres_a1": "Auf meinem Aufgabenblatt erzählt Stefan Baum, ein 45-jähriger IT-Spezialist, dass er mit seiner Familie aufs Land gezogen ist, weil es dort ruhiger, grüner und günstiger als in der Stadt ist.",
            "partner_opin_a1": "Ich kann Stefan sehr gut verstehen. Da ich zwei Kinder habe, mag ich Ruhe und Natur auch sehr gern, aber ein kleiner Vorort in der Nähe einer Stadt ist für mich der beste Kompromiss.",
        },
        "shared_discussion": {
            "OPIN_Q1": "Wie siehst du das? Würdest du lieber mitten in einer großen Stadt wohnen oder ruhig auf dem Land?",
            "OPIN_Q2": "Was gefällt dir persönlich besser: das Leben in der Großstadt oder auf dem Dorf?",
            "HAB_A1": "Hier in Deutschland wohne ich mit meiner Familie in einer Dreizimmerwohnung in Hamburg, aber in einem ruhigen Stadtteil am Rand. So bin ich mit der S-Bahn schnell bei der Arbeit, habe aber trotzdem einen Park in der Nähe.",
            "HAB_Q1": "Wie wohnst du denn zurzeit hier in Deutschland? Wohnst du in einer großen Stadt oder eher ruhig?",
            "HAB_Q2": "Bist du mit deiner aktuellen Wohnsituation zufrieden oder würdest du gern umziehen?",
            "HOME_A1": "In meinem Heimatland Polen ziehen viele junge Leute zum Studieren und Arbeiten in große Städte wie Warschau oder Danzig, weil es auf dem Land oft zu wenige gute Arbeitsplätze gibt.",
            "HOME_Q1": "Und wie ist das in deinem Heimatland? Leben dort die meisten Menschen lieber in den großen Städten oder auf dem Land?",
            "PRO_A1": "In der Großstadt hat man kurze Wege, viele Ärzte, Kultur und gute Busse, aber die Mieten sind extrem hoch. Auf dem Land hat man viel Platz und Ruhe, braucht aber für alles ein Auto.",
            "PRO_Q1": "Was sind für dich die größten Vor- und Nachteile vom Stadtleben im Vergleich zum Landleben?",
        },
        "examiner_followup": {
            "T2_FOLLOW_01": "Darf ich Ihnen noch eine Frage stellen: Glauben Sie, dass durch Homeoffice in Zukunft wieder mehr Menschen aufs Land ziehen werden?",
            "T2_FOLLOW_02": "Noch eine kurze Abschlussfrage: Was ist Ihnen bei einer Wohnung am wichtigsten — die Lage, die Größe oder eine günstige Miete?",
        },
    },
    {
        "topic_id": "T6",
        "persona_key": "P1",  # Leyla & Herr Wagner
        "title": "Soziale Medien & Smartphone im Alltag",
        "card_a": {
            "person": "Alex Müller (23 Jahre, Student)",
            "stance_label": "Pro Soziale Medien & Vernetzung",
            "exam_hint": (
                "Thema: „Soziale Medien & Smartphone im Alltag“\n\n"
                "Ihr Textzettel (Alex Müller, 23 Jahre, Student):\n"
                "„Für mich sind soziale Medien wie Instagram, WhatsApp und TikTok aus dem Alltag nicht mehr wegzudenken. Ich kann damit jeden Tag kostenlos mit meinen Freunden und meiner Familie im Ausland schreiben, Fotos teilen und mich schnell über aktuelle Nachrichten informieren. Außerdem habe ich über soziale Netzwerke schon viele neue Leute mit gleichen Hobbys kennengelernt.“\n\n"
                "Aufgabe:\n"
                "1. Fassen Sie zuerst kurz zusammen, was Alex Müller sagt.\n"
                "2. Diskutieren Sie danach mit Ihrer Partnerin: Ihre eigene Meinung zu sozialen Medien, wie Sie das Smartphone im Alltag nutzen, wie das in Ihrem Heimatland ist und welche Vor- und Nachteile Sie sehen."
            ),
            "partner_t2open": "Gerne! Ich habe einen Text zum Thema soziale Medien gelesen. Die Person heißt Alex Müller, er ist 23 Jahre alt und Student. Er findet soziale Medien super, weil er damit jeden Tag kostenlos Kontakt zu Freunden im Ausland halten, Fotos teilen und schnell aktuelle Nachrichten lesen kann. Und was steht auf deinem Zettel?",
            "partner_pres_a1": "In meinem Text sagt Alex Müller, ein 23-jähriger Student, dass soziale Medien für ihn unverzichtbar sind, um mit Freunden in Kontakt zu bleiben und sich zu informieren.",
            "partner_opin_a1": "Ich sehe das ähnlich wie Alex, besonders weil meine Familie im Ausland lebt. Ohne WhatsApp und Videoanrufe wäre der Kontakt viel schwieriger, auch wenn man aufpassen muss, nicht zu viel Zeit am Handy zu verbringen.",
        },
        "card_b": {
            "person": "Ayşe Özel (43 Jahre, Lehrerin)",
            "stance_label": "Kritisch gegenüber Sozialen Medien",
            "exam_hint": (
                "Thema: „Soziale Medien & Smartphone im Alltag“\n\n"
                "Ihr Textzettel (Ayşe Özel, 43 Jahre, Lehrerin):\n"
                "„Ich sehe soziale Medien sehr kritisch. Als Lehrerin merke ich jeden Tag, dass Jugendliche stundenlang aufs Handy schauen und sich im Unterricht kaum noch konzentrieren können. Viele Menschen treffen sich immer seltener persönlich, vergleichen sich ständig mit anderen im Internet und glauben falschen Nachrichten. Ich nutze mein Smartphone deshalb nur zum Telefonieren.“\n\n"
                "Aufgabe:\n"
                "1. Fassen Sie zuerst kurz zusammen, was Ayşe Özel sagt.\n"
                "2. Diskutieren Sie danach mit Ihrer Partnerin: Ihre eigene Meinung zu sozialen Medien, wie Sie das Smartphone im Alltag nutzen, wie das in Ihrem Heimatland ist und welche Vor- und Nachteile Sie sehen."
            ),
            "partner_t2open": "Gerne! Ich habe einen Text zum Thema soziale Medien gelesen. Die Person heißt Ayşe Özel, sie ist 43 Jahre alt und Lehrerin von Beruf. Sie sieht soziale Medien sehr kritisch, weil viele junge Leute stundenlang aufs Handy schauen, sich schlechter konzentrieren können und sich immer seltener persönlich treffen. Und was steht auf deinem Zettel?",
            "partner_pres_a1": "In meinem Text sagt Ayşe Özel, eine 43-jährige Lehrerin, dass soziale Medien viel Zeit kosten, die Konzentration stören und echte Gespräche ersetzen.",
            "partner_opin_a1": "Ich finde, Frau Özel hat in vielen Punkten recht. Man verliert am Handy schnell die Zeit aus den Augen, aber ganz ohne soziale Medien möchte ich heute trotzdem nicht leben.",
        },
        "shared_discussion": {
            "OPIN_Q1": "Wie denkst du darüber? Sind soziale Medien für dich eher nützlich oder eher ein Problem?",
            "OPIN_Q2": "Was ist deine persönliche Meinung zu sozialen Netzwerken wie Instagram oder WhatsApp?",
            "HAB_A1": "Hier in Deutschland nutze ich jeden Tag WhatsApp und Instagram, vor allem um mit meiner Familie in Izmir zu telefonieren und deutsche Nachrichten oder Lernvideos anzuschauen. Abends lege ich das Handy aber bewusst weg.",
            "HAB_Q1": "Wie nutzt du soziale Medien hier im Alltag? Wie viel Zeit verbringst du ungefähr am Handy?",
            "HAB_Q2": "Welche Apps nutzt du auf deinem Smartphone am häufigsten?",
            "HOME_A1": "In meinem Heimatland, der Türkei, nutzen fast alle Menschen — sogar meine Großeltern — jeden Tag soziale Medien, um Familienfotos zu teilen und Nachrichten zu lesen.",
            "HOME_Q1": "Und wie ist das in deinem Heimatland? Spielen soziale Medien dort auch eine so große Rolle im Alltag?",
            "PRO_A1": "Der größte Vorteil ist definitiv die schnelle und kostenlose Verbindung weltweit. Der größte Nachteil ist aber die Suchtgefahr, der Datenschutz und dass Fake News sich so schnell verbreiten.",
            "PRO_Q1": "Was sind deiner Meinung nach die wichtigsten Vor- und Nachteile von sozialen Medien?",
        },
        "examiner_followup": {
            "T2_FOLLOW_01": "Darf ich noch kurz nachfragen: Sollte es an Schulen ein komplettes Handyverbot für Kinder und Jugendliche geben?",
            "T2_FOLLOW_02": "Noch eine kurze Frage an Sie: Ab welchem Alter sollten Kinder Ihrer Meinung nach ein eigenes Smartphone bekommen?",
        },
    },
    {
        "topic_id": "T7",
        "persona_key": "P2",  # Omar & Frau Fischer
        "title": "Sport & Bewegung im Alltag",
        "card_a": {
            "person": "Maria Lupo (23 Jahre, Studentin)",
            "stance_label": "Pro regelmäßig Sport & Bewegung",
            "exam_hint": (
                "Thema: „Sport & Bewegung im Alltag“\n\n"
                "Ihr Textzettel (Maria Lupo, 23 Jahre, Studentin):\n"
                "„Bewegung ist für mich jeden Tag extrem wichtig. Ich fahre immer mit dem Fahrrad zur Universität, gehe dreimal pro Woche ins Fitnessstudio und am Wochenende jogge ich im Park. Wenn ich keinen Sport mache, bekomme ich sofort Rückenschmerzen und schlechte Laune. Durch den Sport habe ich viel mehr Energie zum Lernen und bleibe gesund.“\n\n"
                "Aufgabe:\n"
                "1. Fassen Sie zuerst kurz zusammen, was Maria Lupo sagt.\n"
                "2. Diskutieren Sie danach mit Ihrem Partner: Ihre eigene Meinung zum Thema Sport, was Sie in Deutschland für Ihre Fitness tun, wie das in Ihrem Heimatland ist und welche Vor- und Nachteile Sport im Verein oder Fitnessstudio hat."
            ),
            "partner_t2open": "Ja, gerne! Mein Text ist zum Thema Bewegung und Sport im Alltag. Maria Lupo ist 23 Jahre alt und Studentin. Sie sagt, dass Bewegung für sie sehr wichtig ist: Sie fährt jeden Tag Rad, geht dreimal pro Woche ins Fitnessstudio und joggt im Park, weil Sport ihr Energie gibt und gegen Rückenschmerzen hilft. Und was steht in deinem Text?",
            "partner_pres_a1": "In meinem Text erzählt Maria Lupo, eine 23-jährige Studentin, dass sie mehrmals pro Woche Sport treibt und mit dem Rad fährt, um fit und ausgeglichen zu bleiben.",
            "partner_opin_a1": "Ich finde Marias Disziplin wirklich toll! Ich merke selbst auch, dass mir regelmäßige Bewegung nach der Arbeit sehr guttut, um den Kopf freizubekommen.",
        },
        "card_b": {
            "person": "Luca Giotto (32 Jahre, Manager)",
            "stance_label": "Keine Zeit für Sport wegen Arbeit",
            "exam_hint": (
                "Thema: „Sport & Bewegung im Alltag“\n\n"
                "Ihr Textzettel (Luca Giotto, 32 Jahre, Manager):\n"
                "„Ich sitze im Beruf zwar den ganzen Tag am Schreibtisch und weiß, dass Bewegung wichtig wäre. Aber nach zehn Stunden Arbeit im Büro bin ich abends einfach viel zu müde für das Fitnessstudio. Meine knappe Freizeit verbringe ich lieber gemütlich mit meiner Familie oder mit Freunden auf dem Sofa. Außerdem sind Fitnessstudios teuer und machen mir keinen Spaß.“\n\n"
                "Aufgabe:\n"
                "1. Fassen Sie zuerst kurz zusammen, was Luca Giotto sagt.\n"
                "2. Diskutieren Sie danach mit Ihrem Partner: Ihre eigene Meinung zum Thema Sport, was Sie in Deutschland für Ihre Fitness tun, wie das in Ihrem Heimatland ist und welche Vor- und Nachteile Sport im Verein oder Fitnessstudio hat."
            ),
            "partner_t2open": "Ja, gerne! Mein Text ist zum Thema Bewegung und Sport im Alltag. Luca Giotto ist 32 Jahre alt und Manager von Beruf. Er arbeitet zehn Stunden am Tag im Büro und sagt, dass er abends viel zu müde für das Fitnessstudio ist und seine Freizeit lieber gemütlich mit seiner Familie verbringt. Und was steht in deinem Text?",
            "partner_pres_a1": "In meinem Text erzählt Luca Giotto, ein 32-jähriger Manager, dass ihm nach der langen Büroarbeit die Zeit und Energie für Sport fehlen.",
            "partner_opin_a1": "Ich kann verstehen, dass man nach der Arbeit müde ist wie Luca. Aber ich finde, man muss nicht unbedingt ins Fitnessstudio gehen — schon ein Spaziergang oder Radfahren hilft sehr.",
        },
        "shared_discussion": {
            "OPIN_Q1": "Wie siehst du das? Ist regelmäßiger Sport für dich wichtig oder fehlt dir im Alltag auch oft die Motivation?",
            "OPIN_Q2": "Was denkst du zum Thema Bewegung im Alltag? Muss man regelmäßig Sport treiben, um gesund zu bleiben?",
            "HAB_A1": "Hier in Deutschland spiele ich einmal pro Woche am Wochenende mit Freunden Fußball und fahre oft mit dem Fahrrad durch die Stadt. Für ein Fitnessstudio fehlt mir unter der Woche meistens die Zeit.",
            "HAB_Q1": "Und wie ist das bei dir hier in Deutschland? Machst du regelmäßig Sport oder bewegst du dich viel im Alltag?",
            "HAB_Q2": "Welchen Sport machst du denn am liebsten, wenn du Freizeit hast?",
            "HOME_A1": "In meinem Heimatland Syrien spielen sehr viele junge Leute draußen Fußball oder gehen abends spazieren. Aber richtige Sportvereine für Erwachsene wie hier in Deutschland sind dort viel seltener.",
            "HOME_Q1": "Wie ist das in deinem Heimatland? Treiben die Menschen dort viel Sport in der Freizeit und welche Sportarten sind beliebt?",
            "PRO_A1": "Der große Vorteil von Sport ist, dass man gesund bleibt, Stress abbaut und im Verein neue Freunde findet. Der Nachteil ist nur, dass Fitnessstudios Geld kosten und man bei Sportverletzungen aufpassen muss.",
            "PRO_Q1": "Was sind deiner Meinung nach die Vor- und Nachteile, wenn man im Fitnessstudio oder im Sportverein trainiert?",
        },
        "examiner_followup": {
            "T2_FOLLOW_01": "Ich hätte noch eine kurze Frage an Sie: Sollten Arbeitgeber ihren Mitarbeitern kostenlose Sportkurse während der Arbeitszeit anbieten?",
            "T2_FOLLOW_02": "Noch eine Frage zum Schluss: Trainieren Sie lieber allein für sich oder lieber gemeinsam in einer Mannschaft?",
        },
    },
    {
        "topic_id": "T8",
        "persona_key": "P3",  # Elena & Frau Erdoğan
        "title": "Rauchen & Gesundheit",
        "card_a": {
            "person": "Max Krüger (40 Jahre, Elektriker)",
            "stance_label": "Raucher aus Gewohnheit & gegen Stress",
            "exam_hint": (
                "Thema: „Rauchen & Rauchverbot in der Öffentlichkeit“\n\n"
                "Ihr Textzettel (Max Krüger, 40 Jahre, Elektriker):\n"
                "„Ich rauche schon seit 20 Jahren, ungefähr eine halbe Schachtel Zigaretten am Tag. Natürlich weiß ich, dass Rauchen ungesund und teuer ist. Aber wenn ich bei der Arbeit viel Stress habe, hilft mir eine kurze Zigarettenpause mit den Kollegen sofort beim Entspannen. Ich finde, jeder Erwachsene sollte selbst entscheiden dürfen, ob er raucht oder nicht.“\n\n"
                "Aufgabe:\n"
                "1. Fassen Sie zuerst kurz zusammen, was Max Krüger sagt.\n"
                "2. Diskutieren Sie danach mit Ihrer Partnerin: Ihre eigene Meinung zum Thema Rauchen, Ihre Erfahrungen in Deutschland, wie die Situation in Ihrem Heimatland ist und was Sie von Rauchverboten halten."
            ),
            "partner_t2open": "Gerne! Mein Text ist über das Thema Rauchen. Bei mir spricht Max Krüger, er ist 40 Jahre alt und Elektriker. Er raucht seit 20 Jahren und weiß zwar, dass es ungesund ist, aber die Zigarettenpause hilft ihm bei Stress auf der Arbeit und er findet, jeder Erwachsene sollte das selbst entscheiden. Und was steht auf deinem Zettel?",
            "partner_pres_a1": "In meinem Text sagt Max Krüger, ein 40-jähriger Elektriker, dass er seit 20 Jahren raucht, weil es ihm bei Stress hilft, obwohl er weiß, dass es ungesund ist.",
            "partner_opin_a1": "Als Krankenschwester sehe ich das natürlich ganz anders als Max. Ich sehe im Krankenhaus jeden Tag, wie gefährlich Rauchen für die Lunge und das Herz ist.",
        },
        "card_b": {
            "person": "Melanie Roth (37 Jahre, Journalistin)",
            "stance_label": "Striktes Nichtrauchen & Rauchverbot",
            "exam_hint": (
                "Thema: „Rauchen & Rauchverbot in der Öffentlichkeit“\n\n"
                "Ihr Textzettel (Melanie Roth, 37 Jahre, Journalistin):\n"
                "„Ich finde Rauchen schrecklich und habe in meinem Leben noch nie eine Zigarette geraucht. Meine Eltern waren starke Raucher und der kalte Rauch in der Wohnung hat mich schon als Kind gestört. Es ist gut, dass Rauchen in Restaurants und Zügen verboten ist, denn Nichtraucher müssen vor Passivrauchen geschützt werden.“\n\n"
                "Aufgabe:\n"
                "1. Fassen Sie zuerst kurz zusammen, was Melanie Roth sagt.\n"
                "2. Diskutieren Sie danach mit Ihrer Partnerin: Ihre eigene Meinung zum Thema Rauchen, Ihre Erfahrungen in Deutschland, wie die Situation in Ihrem Heimatland ist und was Sie von Rauchverboten halten."
            ),
            "partner_t2open": "Gerne! Mein Text ist auch über das Thema Rauchen. Bei mir spricht Melanie Roth, sie ist 37 Jahre alt und Journalistin von Beruf. Sie findet Rauchen schrecklich, hat selbst noch nie geraucht und unterstützt strenge Rauchverbote in Restaurants, damit Nichtraucher geschützt werden. Und was steht auf deinem Zettel?",
            "partner_pres_a1": "In meinem Text sagt Melanie Roth, eine 37-jährige Journalistin, dass sie Rauchen ablehnt und Rauchverbote zum Schutz von Nichtrauchern sehr wichtig findet.",
            "partner_opin_a1": "Ich bin da ganz der gleichen Meinung wie Melanie. Ich arbeite als Krankenschwester und rauche selbst auch überhaupt nicht, weil mir meine Gesundheit sehr wichtig ist.",
        },
        "shared_discussion": {
            "OPIN_Q1": "Wie stehst du zum Thema Rauchen? Rauchst du selbst oder bist du auch gegen das Rauchen?",
            "OPIN_Q2": "Was ist deine persönliche Meinung zum Rauchen und zu Rauchverboten?",
            "HAB_A1": "Hier in Deutschland finde ich es sehr angenehm, dass in allen Restaurants, Cafés und Bahnhöfen Rauchverbot gilt. Mich stört es nur, wenn Leute direkt an der Bushaltestelle im Regen unter dem Dach rauchen.",
            "HAB_Q1": "Was sind deine Erfahrungen hier in Deutschland? Findest du die Regeln für Raucher und Nichtraucher hier gut?",
            "HAB_Q2": "Rauchen in deinem Freundeskreis oder bei deiner Arbeit viele Kollegen?",
            "HOME_A1": "In meinem Heimatland Rumänien wurde früher fast überall geraucht, sogar in Cafés. Inzwischen gibt es dort zum Glück auch ein Rauchverbot in geschlossenen Räumen, aber draußen rauchen immer noch sehr viele Menschen.",
            "HOME_Q1": "Und wie ist die Situation in deinem Heimatland? Rauchen dort viele Menschen und gibt es dort auch strenge Rauchverbote?",
            "PRO_A1": "Raucher sagen oft, dass eine Zigarette kurz gegen Stress hilft und man in der Pause leicht ins Gespräch kommt. Aber die Nachteile sind riesig: Es kostet extrem viel Geld, schadet der Gesundheit und belastet auch andere Menschen durch Passivrauchen.",
            "PRO_Q1": "Warum fangen deiner Meinung nach immer noch so viele junge Leute mit dem Rauchen oder mit E-Zigaretten an?",
        },
        "examiner_followup": {
            "T2_FOLLOW_01": "Darf ich noch kurz fragen: Glauben Sie, dass Zigaretten noch teurer werden sollten, damit weniger Menschen rauchen?",
            "T2_FOLLOW_02": "Noch eine kurze Abschlussfrage: Was halten Sie von E-Zigaretten und Vapes — sollten diese genauso streng geregelt werden?",
        },
    },
]


def build_16_exam_variants():
    """
    Expands the 8 core topics into 16 exam samples:
      - Samples 01..08: Variant A (User gets Card A, Partner presents Card B)
      - Samples 09..16: Variant B (User gets Card B, Partner presents Card A)
    """
    variants = []
    # Pass 1: Variant A (B1_S01 .. B1_S08)
    for idx, t in enumerate(TOPICS, start=1):
        variants.append(_create_variant_entry(idx, t, side="A"))
    # Pass 2: Variant B (B1_S09 .. B1_S16)
    for idx, t in enumerate(TOPICS, start=9):
        variants.append(_create_variant_entry(idx, t, side="B"))
    return variants


def _create_variant_entry(sample_num: int, topic: dict, side: str) -> dict:
    persona = PERSONAS[topic["persona_key"]]
    t_id = topic["topic_id"]          # e.g. "T1"
    p_code = persona["partner_code"]  # e.g. "LEY"
    ex_code = persona["examiner_code"]  # e.g. "EX1"

    # In Variant A: User gets Card A, Partner presents Card B
    # In Variant B: User gets Card B, Partner presents Card A
    if side == "A":
        user_card = topic["card_a"]
        partner_card = topic["card_b"]
    else:
        user_card = topic["card_b"]
        partner_card = topic["card_a"]

    exam_id = f"B1_S{sample_num:02d}"
    sample_folder = f"B1_Exam_Sample_{sample_num:02d}"
    exam_name = (
        f"B1 Exam Sample {sample_num:02d} — {topic['title']} "
        f"({user_card['stance_label']})"
    )

    return {
        "exam_id": exam_id,
        "exam_name": exam_name,
        "sample_folder": sample_folder,
        "teil1_sample_folder": persona["teil1_sample_folder"],
        "topic_id": t_id,
        "side": side,
        "topic_title": topic["title"],
        "partner_short": persona["partner_short"],
        "partner_code": p_code,
        "partner_voice_id": persona["partner_voice_id"],
        "examiner_name": persona["examiner_name"],
        "examiner_code": ex_code,
        "examiner_voice_id": persona["examiner_voice_id"],
        "exam_hint": user_card["exam_hint"],
        "examiner_shared": persona["examiner_shared"],
        "examiner_followup": topic["examiner_followup"],
        "partner_variant_clips": {
            "T2OPEN_01": partner_card["partner_t2open"],
            "PRES_A1": partner_card["partner_pres_a1"],
            "OPIN_A1": partner_card["partner_opin_a1"],
        },
        "partner_topic_shared": topic["shared_discussion"],
    }


def cache_path_for(voice_id: str, text: str) -> str:
    os.makedirs(CACHE_DIR, exist_ok=True)
    key = f"{MODEL_ID}|{voice_id}|{json.dumps(VOICE_SETTINGS, sort_keys=True)}|{text}"
    digest = hashlib.sha256(key.encode("utf-8")).hexdigest()[:24]
    return os.path.join(CACHE_DIR, f"{voice_id}_{digest}.mp3")


def synthesize_to_cache(audio_id: str, text: str, voice_id: str) -> str:
    cached = cache_path_for(voice_id, text)
    if os.path.exists(cached) and os.path.getsize(cached) > 512:
        return f"SKIP (cached): {audio_id} ({os.path.basename(cached)})"

    if not ELEVENLABS_API_KEY:
        raise RuntimeError(
            "ELEVENLABS_API_KEY is not set in environment or .env. "
            "Use --dry-run to generate scripts/manifests without calling ElevenLabs."
        )

    url = f"https://api.elevenlabs.io/v1/text-to-speech/{voice_id}"
    headers = {
        "Accept": "audio/mpeg",
        "Content-Type": "application/json",
        "xi-api-key": ELEVENLABS_API_KEY,
    }
    data = {
        "text": text,
        "model_id": MODEL_ID,
        "language_code": "de",
        "voice_settings": VOICE_SETTINGS,
    }

    for attempt in range(1, 5):
        response = requests.post(url, json=data, headers=headers, timeout=60)
        if response.status_code == 200 and len(response.content) > 256:
            os.makedirs(os.path.dirname(cached), exist_ok=True)
            with open(cached, "wb") as f:
                f.write(response.content)
            return f"NEW ({len(response.content)} B): {audio_id} -> {text[:45]}..."
        print(f"  [WARN] ElevenLabs {response.status_code} for {audio_id} (attempt {attempt}/4): {response.text[:160]}", flush=True)
        time.sleep(1.8 * attempt)

    raise RuntimeError(f"Failed to synthesize {audio_id}")


def build_variant_script_and_tasks(v: dict):
    """
    Builds the `script.json` payload and list of `(audio_id, text, voice_id)` tasks for one of the 16 variants.
    Audio IDs are namespaced to guarantee:
      1. 100% compatibility with Go's `ParseScriptCatalogue` (`parts[2]` = slot, `parts[3]` = kind).
      2. Zero local cache collision in Flutter's `AudioDownloadManager` between different topics/variants.
      3. Maximum cache reuse for identical clips across variants of the same persona/topic.
    """
    p_name = v["partner_short"]
    p_code = v["partner_code"]          # e.g. "LEY"
    p_voice = v["partner_voice_id"]
    ex_name = v["examiner_name"]
    ex_code = v["examiner_code"]        # e.g. "EX1"
    ex_voice = v["examiner_voice_id"]
    t_id = v["topic_id"]                # e.g. "T1"
    side = v["side"]                    # "A" or "B"

    # Namespaced prefixes:
    # - Persona-shared: DE_LEY_... / DE_EX1_...
    # - Topic-shared (shared between Side A and Side B of the same topic): DE_LEYT1_... / DE_EX1T1_...
    # - Variant-specific (flipped between Side A and Side B): DE_LEYT1A_... / DE_LEYT1B_...
    p_topic_prefix = f"DE_{p_code}{t_id}"
    p_var_prefix = f"DE_{p_code}{t_id}{side}"
    ex_topic_prefix = f"DE_{ex_code}{t_id}"

    tasks = []
    dialogues = []

    def add_single(topic_label, speaker, role, audio_id, text, voice_id):
        tasks.append((audio_id, text, voice_id))
        dialogues.append({
            "topic": topic_label,
            "speaker": speaker,
            "role": role,
            "interaction": {
                "dialogue": {
                    "audio_id": audio_id,
                    "text": text,
                }
            },
        })

    def add_qa(topic_label, speaker, role, q_id, q_text, a_id, a_text, voice_id):
        if q_id and q_text:
            tasks.append((q_id, q_text, voice_id))
        if a_id and a_text:
            tasks.append((a_id, a_text, voice_id))
        interaction = {}
        if q_id and q_text:
            interaction["question"] = {"audio_id": q_id, "text": q_text}
        if a_id and a_text:
            interaction["answer"] = {"audio_id": a_id, "text": a_text}
        dialogues.append({
            "topic": topic_label,
            "speaker": speaker,
            "role": role,
            "interaction": interaction,
        })

    # 1. Examiner Intro & Partner Text Presentation (Auto-played at start of Teil 2)
    intro_id = f"DE_{ex_code}_T2_INTRO_01"
    t2open_id = f"{p_var_prefix}_T2OPEN_01"
    add_single("Einleitung Teil 2", ex_name, "Examiner", intro_id, v["examiner_shared"]["T2_INTRO_01"], ex_voice)
    add_single(
        f"Textpräsentation ({v['topic_title']})",
        p_name,
        "Participant",
        t2open_id,
        v["partner_variant_clips"]["T2OPEN_01"],
        p_voice,
    )

    # 2. Text Presentation Slot (PRES)
    add_qa(
        "Textzusammenfassung",
        p_name,
        "Participant",
        f"DE_{p_code}_PRES_Q1",
        SHARED_PARTNER_STRUCTURAL["PRES_Q1"],
        f"{p_var_prefix}_PRES_A1",
        v["partner_variant_clips"]["PRES_A1"],
        p_voice,
    )
    add_qa(
        "Textzusammenfassung (Nachfrage)",
        p_name,
        "Participant",
        f"DE_{p_code}_PRES_Q2",
        SHARED_PARTNER_STRUCTURAL["PRES_Q2"],
        None,
        None,
        p_voice,
    )

    # 3. Dimension 1: Personal Opinion (OPIN)
    td = v["partner_topic_shared"]
    add_qa(
        "Eigene Meinung zum Thema",
        p_name,
        "Participant",
        f"{p_topic_prefix}_OPIN_Q1",
        td["OPIN_Q1"],
        f"{p_var_prefix}_OPIN_A1",
        v["partner_variant_clips"]["OPIN_A1"],
        p_voice,
    )
    add_qa(
        "Eigene Meinung (Alternative)",
        p_name,
        "Participant",
        f"{p_topic_prefix}_OPIN_Q2",
        td["OPIN_Q2"],
        None,
        None,
        p_voice,
    )

    # 4. Dimension 2: Personal Habits & Experiences in Germany (HAB)
    add_qa(
        "Erfahrungen und Gewohnheiten in Deutschland",
        p_name,
        "Participant",
        f"{p_topic_prefix}_HAB_Q1",
        td["HAB_Q1"],
        f"{p_topic_prefix}_HAB_A1",
        td["HAB_A1"],
        p_voice,
    )
    add_qa(
        "Gewohnheiten in Deutschland (Alternative)",
        p_name,
        "Participant",
        f"{p_topic_prefix}_HAB_Q2",
        td["HAB_Q2"],
        None,
        None,
        p_voice,
    )

    # 5. Dimension 3: Home Country Comparison (HOME)
    add_qa(
        "Situation im Heimatland",
        p_name,
        "Participant",
        f"{p_topic_prefix}_HOME_Q1",
        td["HOME_Q1"],
        f"{p_topic_prefix}_HOME_A1",
        td["HOME_A1"],
        p_voice,
    )

    # 6. Dimension 4: Pros & Cons / Deeper Argumentation (PRO)
    add_qa(
        "Vor- und Nachteile",
        p_name,
        "Participant",
        f"{p_topic_prefix}_PRO_Q1",
        td["PRO_Q1"],
        f"{p_topic_prefix}_PRO_A1",
        td["PRO_A1"],
        p_voice,
    )

    # 7. Fallbacks, Clarifications, Silence & Partner Closing
    add_qa("Rückfrage (Meinung)", p_name, "Participant", f"DE_{p_code}_T2BACK_Q1", SHARED_PARTNER_STRUCTURAL["T2BACK_Q1"], None, None, p_voice)
    add_qa("Rückfrage (Erfahrung)", p_name, "Participant", f"DE_{p_code}_T2BACK_Q2", SHARED_PARTNER_STRUCTURAL["T2BACK_Q2"], None, None, p_voice)
    add_qa("Verständnisfrage", p_name, "Participant", f"DE_{p_code}_T2CLAR_Q1", SHARED_PARTNER_STRUCTURAL["T2CLAR_Q1"], None, None, p_voice)
    add_qa("Nachfrage bei Stille", p_name, "Participant", f"DE_{p_code}_T2SIL_Q1", SHARED_PARTNER_STRUCTURAL["T2SIL_Q1"], None, None, p_voice)
    add_single("Abschluss Partner", p_name, "Participant", f"DE_{p_code}_T2CLOSE_01", SHARED_PARTNER_STRUCTURAL["T2CLOSE_01"], p_voice)

    # 8. Modular Expressions (EXP_21 - EXP_38)
    for exp_code, label, text in SHARED_EXPRESSIONS:
        add_single(label, p_name, "Participant", f"DE_{p_code}_{exp_code}", text, p_voice)

    # 9. Examiner Follow-Up Questions, Interventions & Transition Outro to Teil 3
    add_single("Prüfer-Nachfrage 1", ex_name, "Examiner", f"{ex_topic_prefix}_T2_FOLLOW_01", v["examiner_followup"]["T2_FOLLOW_01"], ex_voice)
    add_single("Prüfer-Nachfrage 2", ex_name, "Examiner", f"{ex_topic_prefix}_T2_FOLLOW_02", v["examiner_followup"]["T2_FOLLOW_02"], ex_voice)
    add_single("Prüfer-Intervention: Thema", ex_name, "Examiner", f"DE_{ex_code}_T2_INT_TOPIC", v["examiner_shared"]["T2_INT_TOPIC"], ex_voice)
    add_single("Prüfer-Intervention: Höflichkeit", ex_name, "Examiner", f"DE_{ex_code}_T2_INT_POLITE", v["examiner_shared"]["T2_INT_POLITE"], ex_voice)
    add_single("Prüfer-Intervention: Dialog", ex_name, "Examiner", f"DE_{ex_code}_T2_INT_DIALOG", v["examiner_shared"]["T2_INT_DIALOG"], ex_voice)

    outro_id = f"DE_{ex_code}_T2_OUTRO_01"
    add_single("Abschluss Teil 2", ex_name, "Examiner", outro_id, v["examiner_shared"]["T2_OUTRO_01"], ex_voice)

    script_payload = {
        "exam_id": v["exam_id"],
        "topic_title": v["topic_title"],
        "exam_hint": v["exam_hint"],
        "voice_profiles": {
            p_name: p_voice,
            ex_name: ex_voice,
        },
        "auto_play_sequence": [intro_id, t2open_id],
        "end_audio_id": outro_id,
        "dialogues": dialogues,
    }
    return script_payload, tasks


def update_manifests(variants: list):
    """
    Updates B1_Exams_manifest.json in both output_s3/ and upload_package/telc/b1/speaking/
    so all 16 exam samples (8 topics x 2 sides) are registered.
    - B1_S01..B1_S05 have their native Teil 1 folder + Teil 2 folder.
    - B1_S06..B1_S16 also include their persona's Teil 1 folder (for backward compatibility)
      plus their dedicated Teil 2 folder (B1_Exam_Sample_06/Teil_2/ .. B1_Exam_Sample_16/Teil_2/).
    """
    manifest = {
        "project": "Telc_B1_Speaking_Simulator",
        "exams": [],
    }
    for v in variants:
        t1_folder = v["teil1_sample_folder"]
        t2_folder = v["sample_folder"]
        manifest["exams"].append({
            "id": v["exam_id"],
            "name": v["exam_name"],
            "level": "B1",
            "teils": [
                {
                    "index": 1,
                    "name": "Teil 1",
                    "path": f"{t1_folder}/Teil_1/",
                    "script": "script.json",
                },
                {
                    "index": 2,
                    "name": "Teil 2",
                    "path": f"{t2_folder}/Teil_2/",
                    "script": "script.json",
                },
                {
                    "index": 3,
                    "name": "Teil 3",
                    "path": f"{t1_folder}/Teil_3/",
                    "script": "script.json",
                },
            ],
        })

    for target_dir in [OUTPUT_S3_DIR, UPLOAD_PACKAGE_DIR]:
        os.makedirs(target_dir, exist_ok=True)
        manifest_path = os.path.join(target_dir, "B1_Exams_manifest.json")
        with open(manifest_path, "w", encoding="utf-8") as f:
            json.dump(manifest, f, indent=2, ensure_ascii=False)
        print(f"[MANIFEST] Updated {manifest_path} with {len(variants)} exams (8 topics x 2 variants).")


def main():
    parser = argparse.ArgumentParser(description="Generate Telc B1 Teil 2 Audio & Scripts (8 Topics x 2 Variants = 16 Scenarios)")
    parser.add_argument("--dry-run", action="store_true", help="Write script.json and manifest files without calling ElevenLabs")
    args = parser.parse_args()

    variants = build_16_exam_variants()
    variant_packages = []
    unique_tasks = {}  # audio_id -> (text, voice_id)
    total_files_referenced = 0

    for v in variants:
        script_payload, tasks = build_variant_script_and_tasks(v)
        variant_packages.append((v, script_payload, tasks))
        total_files_referenced += len(tasks)
        for audio_id, text, voice_id in tasks:
            unique_tasks[audio_id] = (text, voice_id)

    if not args.dry_run:
        print(f"Synthesizing {len(unique_tasks)} unique Teil 2 audio clips via ElevenLabs ({MODEL_ID})...", flush=True)
        completed = 0
        errors = []
        with ThreadPoolExecutor(max_workers=3) as pool:
            future_map = {
                pool.submit(synthesize_to_cache, audio_id, text, voice_id): audio_id
                for audio_id, (text, voice_id) in unique_tasks.items()
            }
            for fut in as_completed(future_map):
                aid = future_map[fut]
                completed += 1
                try:
                    status = fut.result()
                    print(f"[{completed:03d}/{len(unique_tasks):03d}] {status}", flush=True)
                except Exception as exc:
                    print(f"[{completed:03d}/{len(unique_tasks):03d}] ERROR {aid}: {exc}", flush=True)
                    errors.append((aid, str(exc)))
        if errors:
            raise RuntimeError(f"Encountered {len(errors)} synthesis errors: {errors[:5]}")

    for v, script_payload, tasks in variant_packages:
        folder = v["sample_folder"]
        s3_teil2_dir = os.path.join(OUTPUT_S3_DIR, folder, "Teil_2")
        pkg_teil2_dir = os.path.join(UPLOAD_PACKAGE_DIR, folder, "Teil_2")
        os.makedirs(s3_teil2_dir, exist_ok=True)
        os.makedirs(pkg_teil2_dir, exist_ok=True)

        if not args.dry_run:
            for audio_id, text, voice_id in tasks:
                cached = cache_path_for(voice_id, text)
                s3_mp3 = os.path.join(s3_teil2_dir, f"{audio_id}.mp3")
                pkg_mp3 = os.path.join(pkg_teil2_dir, f"{audio_id}.mp3")
                shutil.copyfile(cached, s3_mp3)
                shutil.copyfile(cached, pkg_mp3)

        for target_dir in [s3_teil2_dir, pkg_teil2_dir]:
            script_path = os.path.join(target_dir, "script.json")
            with open(script_path, "w", encoding="utf-8") as f:
                json.dump(script_payload, f, indent=2, ensure_ascii=False)

    update_manifests(variants)
    print(
        f"\nDone! Processed {len(variants)} Teil 2 exam scenarios "
        f"({total_files_referenced} total clip slots; {len(unique_tasks)} unique MP3 clips synthesized; dry_run={args.dry_run}).",
        flush=True,
    )


if __name__ == "__main__":
    main()
