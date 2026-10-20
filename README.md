# Voice Exam Simulator Backend (Go)

This is the Go backend for the telc German Exam Simulator. It uses a "Glass Pipe" architecture to provide real-time STT (Groq), LLM (Groq), and TTS (ElevenLabs) via WebSockets.

## 🛠️ Local Development

1.  **Configure environment**:
    ```bash
    cp .env.example .env
    # Edit .env with your API keys
    ```
2.  **Run Locally**:
    ```bash
    ./run.sh
    ```
    This script will automatically fall back to running via Docker if you don't have Go installed locally.

## 🚀 Deployment (Lightsail VPS)

This project is optimized to run on a 1 vCPU / 1GB AWS Lightsail instance (Amazon Linux 2023).

### 1. Prerequisites

1. Create a Lightsail instance.
2. Attach a static IP to the instance.
3. Open ports **80** and **443** in the IPv4 Firewall (do not open 8080).
4. Point your domain to the static IP.
5. Get your SSH key (e.g. `LightsailDefaultKey-eu-central-1.pem`).
6. Put your `.env` file containing all the production secrets in this directory.

### 2. Setup (One-time)

Run the script to generate `deploy.env`:
```bash
./deploy.sh
```
Fill out `deploy.env` with your VPS IP, user (`ec2-user`), SSH key path, and domain.

Then run the one-time setup (installs Docker, Caddy, 1GB Swap):
```bash
./deploy.sh setup
```

### 3. Deploy

To build the image and ship it to the VPS:
```bash
./deploy.sh
```

### Useful commands
- `./deploy.sh logs`: Tail the container logs.
- `./deploy.sh rollback`: Revert to the previously deployed image.
