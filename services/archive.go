package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"

	"ai_exam_simulator/config"
)

type SessionMetadata struct {
	Transcription string `json:"transcription"`
	LLMResponse   string `json:"llm_response"`
	AudioS3Key    string `json:"audio_s3_key,omitempty"`
}

type ArchiveJob struct {
	SessionID  string
	Metadata   SessionMetadata
	AudioOut   []byte // examiner TTS MP3
	AudioIn    []byte // learner STT WAV
}

type Archiver interface {
	Enqueue(job ArchiveJob)
	Close()
}

type noopArchiver struct{}

func (noopArchiver) Enqueue(ArchiveJob) {}
func (noopArchiver) Close()             {}

type s3Archiver struct {
	client     *s3.Client
	bucket     string
	queue      chan ArchiveJob
	wg         sync.WaitGroup
	log        *slog.Logger
	saveInput  bool
}

func NewArchiver(ctx context.Context, cfg config.Config, logger *slog.Logger) Archiver {
	if cfg.S3BucketName == "" {
		return noopArchiver{}
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.AWSRegion))
	if err != nil {
		logger.Error("failed to load AWS config, disabling archiver", "error", err)
		return noopArchiver{}
	}

	a := &s3Archiver{
		client:    s3.NewFromConfig(awsCfg),
		bucket:    cfg.S3BucketName,
		queue:     make(chan ArchiveJob, cfg.ArchiveQueueSize),
		log:       logger.With("component", "archiver"),
		saveInput: cfg.ArchiveInputAudio,
	}

	for i := 0; i < cfg.ArchiveWorkers; i++ {
		a.wg.Add(1)
		go a.worker()
	}

	return a
}

func (a *s3Archiver) Enqueue(job ArchiveJob) {
	select {
	case a.queue <- job:
	default:
		a.log.Warn("archive queue full, dropping job", "session_id", job.SessionID)
	}
}

func (a *s3Archiver) Close() {
	close(a.queue)
	a.wg.Wait()
}

func (a *s3Archiver) worker() {
	defer a.wg.Done()
	for job := range a.queue {
		a.process(job)
	}
}

func (a *s3Archiver) process(job ArchiveJob) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	now := time.Now().UTC().Format("20060102_150405")
	shortUUID := uuid.New().String()[:8]
	baseKey := fmt.Sprintf("sessions/%s/%s_%s", job.SessionID, now, shortUUID)

	if len(job.AudioOut) > 0 {
		outKey := baseKey + ".mp3"
		if err := a.upload(ctx, outKey, job.AudioOut, "audio/mpeg"); err == nil {
			job.Metadata.AudioS3Key = outKey
		}
	}

	if a.saveInput && len(job.AudioIn) > 0 {
		inKey := baseKey + ".input.wav"
		_ = a.upload(ctx, inKey, job.AudioIn, "audio/wav")
	}

	metaBytes, err := json.Marshal(job.Metadata)
	if err == nil {
		metaKey := baseKey + ".json"
		_ = a.upload(ctx, metaKey, metaBytes, "application/json")
	}

	a.log.Info("session archived", "base_key", baseKey)
}

func (a *s3Archiver) upload(ctx context.Context, key string, data []byte, contentType string) error {
	_, err := a.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(a.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		a.log.Error("s3 upload failed", "key", key, "error", err)
	}
	return err
}
