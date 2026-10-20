#!/usr/bin/env python3
"""
Prepares the unified upload package:
upload_package/
└── telc/
    └── b1/
        ├── speaking/
        │   ├── B1_Exams_manifest.json
        │   ├── B1_Exam_Sample_01/
        │   ├── B1_Exam_Sample_02/
        │   ├── B1_Exam_Sample_03/
        │   ├── B1_Exam_Sample_04/
        │   └── B1_Exam_Sample_05/
        └── listening/
            ├── b1.json
            └── [all B1 listening mp3 files...]
"""

import os
import shutil
import urllib.request
import json
from pathlib import Path
from concurrent.futures import ThreadPoolExecutor, as_completed

BASE_DIR = Path(__file__).resolve().parent
OUTPUT_S3_DIR = BASE_DIR / "output_s3"
UPLOAD_ROOT = BASE_DIR / "upload_package"
TELC_B1_DIR = UPLOAD_ROOT / "telc" / "b1"
SPEAKING_DIR = TELC_B1_DIR / "speaking"
LISTENING_DIR = TELC_B1_DIR / "listening"

LISTENING_REMOTE_BASE = "https://lingua-stories-audio-exam.s3.eu-central-1.amazonaws.com"


def prepare_speaking():
    print(">>> Copying speaking exam assets...")
    if SPEAKING_DIR.exists():
        shutil.rmtree(SPEAKING_DIR)
    shutil.copytree(OUTPUT_S3_DIR, SPEAKING_DIR)
    total_files = sum(1 for _ in SPEAKING_DIR.rglob("*") if _.is_file())
    print(f"Copied {total_files} speaking files into {SPEAKING_DIR}")


def download_file(filename: str):
    dst = LISTENING_DIR / filename
    if dst.exists() and dst.stat().st_size > 0:
        return f"SKIP (exists): {filename}"

    url = f"{LISTENING_REMOTE_BASE}/{filename}"
    try:
        urllib.request.urlretrieve(url, dst)
        return f"OK: {filename} ({dst.stat().st_size} B)"
    except Exception as e:
        return f"ERROR {filename}: {e}"


def prepare_listening():
    print(">>> Fetching B1 listening exam assets from lingua-stories-audio-exam...")
    LISTENING_DIR.mkdir(parents=True, exist_ok=True)

    # 1. Download b1.json
    b1_json_path = LISTENING_DIR / "b1.json"
    urllib.request.urlretrieve(f"{LISTENING_REMOTE_BASE}/b1.json", b1_json_path)
    print("Downloaded b1.json")

    # 2. Parse all referenced MP3 files
    data = json.loads(b1_json_path.read_text(encoding="utf-8"))
    files_to_download = set()
    for exam in data:
        if exam.get("audioURL"):
            files_to_download.add(exam["audioURL"])
        for teil in exam.get("teile", []):
            if teil.get("audioURL"):
                files_to_download.add(teil["audioURL"])
            if teil.get("beispielAudioURL"):
                files_to_download.add(teil["beispielAudioURL"])

    print(f"Found {len(files_to_download)} listening audio files to download...")

    # Parallel download
    with ThreadPoolExecutor(max_workers=8) as pool:
        futures = {pool.submit(download_file, f): f for f in files_to_download}
        completed = 0
        for fut in as_completed(futures):
            completed += 1
            res = fut.result()
            print(f"[{completed:02d}/{len(files_to_download):02d}] {res}")

    total_files = sum(1 for _ in LISTENING_DIR.glob("*") if _.is_file())
    print(f"Prepared {total_files} files in {LISTENING_DIR}")


def main():
    TELC_B1_DIR.mkdir(parents=True, exist_ok=True)
    prepare_speaking()
    prepare_listening()

    print("\n==========================================")
    print("Upload package ready at:")
    print(f"  {UPLOAD_ROOT}")
    print("Structure:")
    print("  upload_package/")
    print("  └── telc/")
    print("      └── b1/")
    print("          ├── speaking/   (5 samples + manifest + 300 mp3s)")
    print("          └── listening/  (b1.json + 60 mp3s)")
    print("==========================================")


if __name__ == "__main__":
    main()
