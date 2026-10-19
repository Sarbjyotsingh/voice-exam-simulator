import os
import sys
import time
import json
import re
import subprocess
import requests
from concurrent.futures import ThreadPoolExecutor, as_completed

ASSEMBLY_AI_API_KEY = "99afa464cb6d41c2991267ea8df4ca76"
OUTPUT_DIR = "/Users/sarbjyot/StudioProjects/lingua_stories/telc_b1_transcripts"
TEMP_AUDIO_DIR = "/Users/sarbjyot/StudioProjects/lingua_stories/temp_telc_audio"

VIDEOS = [
    ("lDdpsWBhsTk", "https://www.youtube.com/watch?v=lDdpsWBhsTk"),
    ("ACHLdjvlBk0", "https://www.youtube.com/watch?v=ACHLdjvlBk0"),
    ("QMCvJyOu2G8", "https://www.youtube.com/watch?v=QMCvJyOu2G8"),
    ("yLHfjh7PSnI", "https://www.youtube.com/watch?v=yLHfjh7PSnI"),
    ("ty7BGMWfb3U", "https://www.youtube.com/watch?v=ty7BGMWfb3U"),
    ("522olaTKdkg", "https://www.youtube.com/watch?v=522olaTKdkg"),
    ("O9W79EGCvfk", "https://www.youtube.com/watch?v=O9W79EGCvfk"),
    ("ENtPq1T58sc", "https://www.youtube.com/watch?v=ENtPq1T58sc"),
    ("w8ZSTOWxFJo", "https://www.youtube.com/watch?v=w8ZSTOWxFJo"),
    ("O3-nv8_lA-U", "https://www.youtube.com/watch?v=O3-nv8_lA-U"),
    ("PKb0dMfEGRE", "https://www.youtube.com/watch?v=PKb0dMfEGRE"),
    ("nPHbRSaljfs", "https://www.youtube.com/watch?v=nPHbRSaljfs"),
    ("v9ilBJ8jglI", "https://www.youtube.com/watch?v=v9ilBJ8jglI"),
    ("F1BFjB_uGWw", "https://www.youtube.com/watch?v=F1BFjB_uGWw"),
    ("6DQ7griuQEw", "https://www.youtube.com/watch?v=6DQ7griuQEw"),
    ("i2MSIcT_ygA", "https://www.youtube.com/watch?v=i2MSIcT_ygA"),
    ("efuFNArdqRg", "https://www.youtube.com/watch?v=efuFNArdqRg"),
    ("svZPnSNWCWc", "https://www.youtube.com/watch?v=svZPnSNWCWc"),
]

def ms_to_timestamp(ms):
    if ms is None:
        return "00:00:00"
    total_seconds = int(ms) // 1000
    hours = total_seconds // 3600
    minutes = (total_seconds % 3600) // 60
    seconds = total_seconds % 60
    if hours > 0:
        return f"{hours:02d}:{minutes:02d}:{seconds:02d}"
    return f"{minutes:02d}:{seconds:02d}"

def upload_audio_to_assemblyai(file_path, vid):
    headers = {'authorization': ASSEMBLY_AI_API_KEY}
    def file_generator():
        with open(file_path, 'rb') as f:
            while chunk := f.read(5242880): # 5MB chunk
                yield chunk
    size_mb = os.path.getsize(file_path) / (1024 * 1024)
    print(f"[{vid}] Uploading audio ({size_mb:.2f} MB) to AssemblyAI...")
    res = requests.post("https://api.assemblyai.com/v2/upload", headers=headers, data=file_generator())
    res.raise_for_status()
    upload_url = res.json()["upload_url"]
    print(f"[{vid}] Upload complete!")
    return upload_url

def request_transcription(upload_url, vid):
    headers = {
        'authorization': ASSEMBLY_AI_API_KEY,
        'content-type': 'application/json'
    }
    payload = {
        "audio_url": upload_url,
        "language_code": "de",
        "speaker_labels": True,
        "punctuate": True,
        "format_text": True,
        "speech_models": ["universal-3-5-pro", "universal-2"]
    }
    res = requests.post("https://api.assemblyai.com/v2/transcript", headers=headers, json=payload)
    res.raise_for_status()
    transcript_id = res.json()["id"]
    print(f"[{vid}] Transcription queued: {transcript_id}")
    return transcript_id

def poll_transcription(transcript_id, vid):
    headers = {'authorization': ASSEMBLY_AI_API_KEY}
    endpoint = f"https://api.assemblyai.com/v2/transcript/{transcript_id}"
    print(f"[{vid}] Waiting for transcription to finish...")
    attempts = 0
    while True:
        res = requests.get(endpoint, headers=headers)
        res.raise_for_status()
        data = res.json()
        status = data.get("status")
        if status == "completed":
            print(f"[{vid}] Transcription completed successfully!")
            return data
        elif status == "error":
            error_msg = data.get("error", "Unknown error")
            print(f"[{vid}] Transcription error: {error_msg}")
            raise RuntimeError(f"Transcription failed for {vid}: {error_msg}")
        else:
            time.sleep(6)
            attempts += 1
            if attempts % 5 == 0:
                print(f"[{vid}] Still transcribing ({status})...")

def format_markdown(video_info, transcript_data):
    vid = video_info["id"]
    title = video_info.get("title", vid)
    url = video_info["url"]
    duration = video_info.get("duration", "Unknown")
    
    utterances = transcript_data.get("utterances") or []
    full_text = transcript_data.get("text") or ""
    
    speakers = sorted(list(set(u.get("speaker") for u in utterances if u.get("speaker"))))
    
    speaker_stats = {}
    for u in utterances:
        spk = u.get("speaker", "Unknown")
        spk_time = (u.get("end", 0) - u.get("start", 0)) / 1000
        if spk not in speaker_stats:
            speaker_stats[spk] = {"count": 0, "total_seconds": 0}
        speaker_stats[spk]["count"] += 1
        speaker_stats[spk]["total_seconds"] += spk_time
        
    dur_int = 0
    if isinstance(duration, (int, float)):
        dur_int = int(duration)
    elif isinstance(duration, str) and duration.isdigit():
        dur_int = int(duration)

    md = []
    md.append(f"# Telc B1 Mündliche Prüfung: {title}\n")
    md.append(f"- **YouTube URL:** {url}")
    md.append(f"- **Video ID:** `{vid}`")
    md.append(f"- **Dauer:** {ms_to_timestamp(dur_int * 1000)} ({duration} Sekunden)")
    md.append(f"- **Sprache:** Deutsch (`de`)")
    md.append(f"- **Erkannte Sprecher:** {len(speakers)} ({', '.join([f'Speaker {s}' for s in speakers]) if speakers else 'Keine Diarisierung'})")
    md.append(f"- **Anzahl Sprechbeiträge:** {len(utterances)}")
    md.append(f"- **AssemblyAI Transkript-ID:** `{transcript_data.get('id')}`\n")
    
    if speaker_stats:
        md.append("### Sprecher-Statistik\n")
        md.append("| Sprecher | Wortmeldungen / Beiträge | Gesamtsprechzeit |")
        md.append("|---|---|---|")
        for spk, stats in sorted(speaker_stats.items()):
            time_str = ms_to_timestamp(int(stats["total_seconds"] * 1000))
            md.append(f"| **Speaker {spk}** | {stats['count']} | {time_str} |")
        md.append("\n")
        
    md.append("---\n")
    md.append("## Strukturierte Konversation (Mit Sprecher-Kennzeichnung & Zeitstempeln)\n")
    
    if utterances:
        for u in utterances:
            speaker = u.get("speaker", "Unknown")
            start = u.get("start")
            end = u.get("end")
            text = u.get("text", "").strip()
            
            start_str = ms_to_timestamp(start)
            end_str = ms_to_timestamp(end)
            md.append(f"**[{start_str} - {end_str}] Speaker {speaker}:**")
            md.append(f"{text}\n")
    else:
        md.append(full_text)
        md.append("\n")
        
    md.append("\n---\n")
    md.append("## Vollständiger Fließtext\n")
    md.append(f"{full_text}\n")
    
    return "\n".join(md)

def process_single_video(vid, url, idx, total):
    print(f"\n>>> Starting [{idx}/{total}] : {vid} ({url})")
    
    json_path = os.path.join(OUTPUT_DIR, f"{vid}.json")
    md_path = os.path.join(OUTPUT_DIR, f"{vid}.md")
    
    # Metadata fetch
    meta_cmd = ['yt-dlp', '--print', '%(title)s\t%(duration)s', '--no-warnings', url]
    meta_res = subprocess.run(meta_cmd, capture_output=True, text=True)
    if meta_res.returncode == 0 and '\t' in meta_res.stdout:
        parts = meta_res.stdout.strip().split('\t', 1)
        title = parts[0]
        duration = parts[1] if len(parts) > 1 else "Unknown"
    else:
        title = vid
        duration = "Unknown"
        
    video_info = {
        "id": vid,
        "url": url,
        "title": title,
        "duration": duration
    }
    
    # Check if already processed (both json and md exist)
    if os.path.exists(json_path) and os.path.exists(md_path):
        print(f"[{vid}] Already completed! Skipping re-transcription.")
        return video_info
        
    # If JSON already exists (e.g. from previous run), just generate markdown
    if os.path.exists(json_path):
        print(f"[{vid}] JSON exists, generating markdown...")
        with open(json_path, 'r', encoding='utf-8') as f:
            transcript_data = json.load(f)
        md_content = format_markdown(video_info, transcript_data)
        with open(md_path, 'w', encoding='utf-8') as f:
            f.write(md_content)
        return video_info
        
    # Download audio stream directly (no ffmpeg needed)
    os.makedirs(TEMP_AUDIO_DIR, exist_ok=True)
    audio_template = os.path.join(TEMP_AUDIO_DIR, f"{vid}.%(ext)s")
    
    print(f"[{vid}] Downloading audio stream...")
    dl_cmd = [
        'yt-dlp',
        '-f', 'ba[ext=m4a]/ba',
        '--no-warnings',
        '-o', audio_template,
        url
    ]
    subprocess.run(dl_cmd, check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    
    candidates = [os.path.join(TEMP_AUDIO_DIR, f) for f in os.listdir(TEMP_AUDIO_DIR) if f.startswith(vid)]
    if not candidates:
        raise FileNotFoundError(f"Audio file for {vid} not found in {TEMP_AUDIO_DIR}!")
    audio_file = candidates[0]
    
    try:
        # Upload
        upload_url = upload_audio_to_assemblyai(audio_file, vid)
    finally:
        # USER REQUIREMENT: Delete video/audio immediately!
        if os.path.exists(audio_file):
            print(f"[{vid}] Deleting downloaded audio file...")
            os.remove(audio_file)
            
    # Submit transcription
    transcript_id = request_transcription(upload_url, vid)
    
    # Poll until ready
    transcript_data = poll_transcription(transcript_id, vid)
    transcript_data["video_info"] = video_info
    
    # Save JSON
    with open(json_path, 'w', encoding='utf-8') as f:
        json.dump(transcript_data, f, ensure_ascii=False, indent=2)
    print(f"[{vid}] Saved JSON transcript -> {json_path}")
    
    # Save Markdown
    md_content = format_markdown(video_info, transcript_data)
    with open(md_path, 'w', encoding='utf-8') as f:
        f.write(md_content)
    print(f"[{vid}] Saved Markdown transcript -> {md_path}")
    
    return video_info

def generate_index(processed_videos):
    index_path = os.path.join(OUTPUT_DIR, "README.md")
    lines = [
        "# TELC Deutsch B1 Prüfungen - Transkripte Übersicht\n",
        "Diese Sammlung enthält vollständige, strukturierte Transkripte von 18 B1 Telc Prüfungs-Videos.",
        "Alle Transkripte wurden mit der AssemblyAI API mit Diarisierung und Zeitstempeln verarbeitet.\n",
        "### Features:",
        "- **Sprecher-Diarisierung (Speaker Diarization):** Automatische Zuordnung der Sprecher (Prüfer A/B und Kandidat 1/2)",
        "- **Präzise Zeitstempel:** Start- und Endzeiten für jeden Sprecherwechsel im Format `[MM:SS - MM:SS]`",
        "- **Strukturierte Markdown-Dateien (`.md`):** Ideal zur Analyse von Prüfungsphasen (Teil 1, Teil 2, Teil 3)",
        "- **Vollständige JSON-Rohdaten (`.json`):** Mit Wort-Ebene, Konfidenzwerten und Timestamps",
        "- **Keine verbleibenden Video-/Audiodateien:** Temporäre Mediendateien wurden direkt nach dem Upload gelöscht.\n",
        "## Übersicht aller 18 Transkripte\n",
        "| # | Titel | Video ID | Dauer | Markdown Transkript | JSON Daten |",
        "|---|---|---|---|---|---|"
    ]
    
    for i, v in enumerate(processed_videos, 1):
        if not v:
            continue
        vid = v["id"]
        title = v.get("title", vid).replace("|", "-")
        duration = v.get("duration", "N/A")
        if str(duration).isdigit():
            dur_str = ms_to_timestamp(int(duration) * 1000)
        else:
            dur_str = duration
        lines.append(f"| {i} | [{title}]({v['url']}) | `{vid}` | {dur_str} | [Transkript ({vid}.md)](./{vid}.md) | [JSON ({vid}.json)](./{vid}.json) |")
        
    lines.append("\n## Telc B1 Mündliche Prüfung - Typische Phasen & Struktur")
    lines.append("1. **Teil 1: Sich vorstellen (Kontaktaufnahme - ca. 3 Min.)**")
    lines.append("   - Name, Herkunft, Wohnort, Familie, Beruf/Ausbildung, Sprachen, Hobbys.")
    lines.append("   - Der Prüfer stellt anschließend 1-2 vertiefende Fragen (z.B. Nachbuchstabieren, Telefonnummer, Details zum Beruf).")
    lines.append("2. **Teil 2: Ein Gespräch über ein Thema führen (ca. 6 Min.)**")
    lines.append("   - Jeder Teilnehmer berichtet über eigene Erfahrungen zu einem vorgegebenen Thema (z.B. Urlaub, Haustiere, Verkehrsmittel, Gesunde Ernährung).")
    lines.append("   - Die Teilnehmer stellen sich gegenseitig Fragen zum Vortrag.")
    lines.append("3. **Teil 3: Gemeinsam eine Aufgabe lösen (ca. 6 Min.)**")
    lines.append("   - Beide Teilnehmer planen eine konkrete Aktivität zusammen (z.B. eine Abschiedsparty, ein Ausflug für den Deutschkurs, ein Geschenk kaufen).")
    lines.append("   - Vorschläge machen, zustimmen/ablehnen, Termine vereinbaren, Aufgaben verteilen.\n")
    
    with open(index_path, 'w', encoding='utf-8') as f:
        f.write("\n".join(lines))
    print(f"\nGenerated index: {index_path}")

def main():
    os.makedirs(OUTPUT_DIR, exist_ok=True)
    os.makedirs(TEMP_AUDIO_DIR, exist_ok=True)
    
    total = len(VIDEOS)
    print(f"Starting batch transcription of {total} Telc B1 videos using AssemblyAI...")
    print(f"Target directory: {OUTPUT_DIR}")
    
    processed_videos = [None] * total
    
    # Process with 3 concurrent workers to be fast yet safe
    with ThreadPoolExecutor(max_workers=3) as executor:
        future_to_idx = {
            executor.submit(process_single_video, vid, url, idx + 1, total): idx
            for idx, (vid, url) in enumerate(VIDEOS)
        }
        for future in as_completed(future_to_idx):
            idx = future_to_idx[future]
            vid, url = VIDEOS[idx]
            try:
                info = future.result()
                processed_videos[idx] = info
            except Exception as e:
                print(f"ERROR processing video {vid}: {e}")
                processed_videos[idx] = {
                    "id": vid,
                    "url": url,
                    "title": vid,
                    "duration": "Error"
                }
                
    generate_index(processed_videos)
    
    # Cleanup temp audio dir
    if os.path.exists(TEMP_AUDIO_DIR):
        try:
            for f in os.listdir(TEMP_AUDIO_DIR):
                os.remove(os.path.join(TEMP_AUDIO_DIR, f))
            os.rmdir(TEMP_AUDIO_DIR)
        except Exception:
            pass
            
    print(f"\n=== FINISHED! All {total} videos processed. Transcripts in: {OUTPUT_DIR} ===")

if __name__ == "__main__":
    main()
