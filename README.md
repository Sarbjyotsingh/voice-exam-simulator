# AI Exam Simulator Backend

This is the FastAPI backend for the telc German Exam Simulator. It uses a "Glass Pipe" architecture to provide real-time STT (Groq), LLM (Groq), and TTS (ElevenLabs) via WebSockets.

## 🚀 Deployment Guide (AWS ECS Express Mode)

Use these settings when deploying via the AWS ECS "Express Mode" wizard.

### 1. Networking & Port

- **Container Port**: `80`
- **Health Check Path**: `/health`

### 2. Environment Variables

Add these in the **Environment Variables** section:

- `GROQ_API_KEY`: Your Groq API Key.
- `ELEVENLABS_API_KEY`: Your ElevenLabs API Key.
- `S3_BUCKET_NAME`: `telc-exam-simulator-sessions` (For archival).
- `AWS_REGION`: `eu-central-1`

### 3. Command

- uvicorn,app.main:app,--host,0.0.0.0,--port,80,--proxy-headers

### 4. IAM Roles

- **Task Execution Role**: `ecsTaskExecutionRole`
- **Task Role**: Ensure this role has `AmazonS3FullAccess` or a custom policy allowing `s3:PutObject` to your bucket.

### 5. Compute Resources

- **CPU**: `.5 vCPU` (512 units)
- **Memory**: `1 GB`

### 6. Summary Table

| Field            | Value                        |
| :--------------- | :--------------------------- |
| **Port**         | `80`                         |
| **Health Check** | `/health`                    |
| **Auto Scaling** | Min: 1, Max: 3               |
| **Protocol**     | `wss://` (Secure WebSockets) |

---

## 🛠️ Local Development

1.  **Activate Venv**:
    ```bash
    source venv/bin/activate
    ```
2.  **Run Locally**:
    ```bash
    uvicorn app.main:app --reload --port 8000
    ```
3.  **Test AWS Endpoint**:
    ```bash
    python scratch/test_aws_ws.py
    ```

## 📦 Pushing Updates

To build and push a new version to ECR, run:

```bash
sh deploy.sh
```

> [!TIP]
> After pushing a new image, you may need to go to the ECS Console and click **"Deploy new revision"** to force the service to pull the latest `:latest` image.
