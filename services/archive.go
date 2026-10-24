package services

import (
	"bytes"
	"context"
	"fmt"
	"hash/fnv"
	"log/slog"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"voice-engine/config"
)

// File is one object to write to the (write-only) log bucket.
type File struct {
	Key         string
	Body        []byte
	ContentType string
}

// Batch is an ordered set of files belonging to one user. Batches for the same user are
// always uploaded in the order they were enqueued (summaries are cumulative, so a stale
// summary must never overwrite a newer one).
type Batch struct {
	UserID string
	Files  []File
}

type Archiver interface {
	Enqueue(b Batch)
	Close()
}

type noopArchiver struct{}

func (noopArchiver) Enqueue(Batch) {}
func (noopArchiver) Close()        {}

type s3Archiver struct {
	client *s3.Client
	bucket string
	queues []chan Batch
	wg     sync.WaitGroup
	log    *slog.Logger
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

	workers := cfg.ArchiveWorkers
	if workers < 1 {
		workers = 1
	}
	a := &s3Archiver{
		client: s3.NewFromConfig(awsCfg),
		bucket: cfg.S3BucketName,
		log:    logger.With("component", "archiver"),
	}
	// One queue per worker; users are sharded onto a queue by hash so per-user ordering holds.
	for i := 0; i < workers; i++ {
		q := make(chan Batch, cfg.ArchiveQueueSize)
		a.queues = append(a.queues, q)
		a.wg.Add(1)
		go a.worker(q)
	}
	return a
}

func (a *s3Archiver) Enqueue(b Batch) {
	h := fnv.New32a()
	_, _ = h.Write([]byte(b.UserID))
	q := a.queues[int(h.Sum32())%len(a.queues)]
	select {
	case q <- b:
	default:
		// The ledger on disk already holds the authoritative totals and the next batch carries
		// fresh cumulative summaries, so a dropped batch only loses that turn's raw files.
		a.log.Warn("archive queue full, dropping batch", "user_id", b.UserID, "files", len(b.Files))
	}
}

func (a *s3Archiver) Close() {
	for _, q := range a.queues {
		close(q)
	}
	a.wg.Wait()
}

func (a *s3Archiver) worker(q chan Batch) {
	defer a.wg.Done()
	for b := range q {
		a.process(b)
	}
}

func (a *s3Archiver) process(b Batch) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	failed := 0
	for _, f := range b.Files {
		if err := a.upload(ctx, f); err != nil {
			failed++
		}
	}
	if failed > 0 {
		a.log.Warn("batch partially uploaded", "user_id", b.UserID, "failed", failed, "total", len(b.Files))
		return
	}
	a.log.Info("batch archived", "user_id", b.UserID, "files", len(b.Files))
}

func (a *s3Archiver) upload(ctx context.Context, f File) error {
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		_, lastErr = a.client.PutObject(ctx, &s3.PutObjectInput{
			Bucket:      aws.String(a.bucket),
			Key:         aws.String(f.Key),
			Body:        bytes.NewReader(f.Body),
			ContentType: aws.String(f.ContentType),
		})
		if lastErr == nil {
			return nil
		}
		time.Sleep(time.Duration(attempt) * 300 * time.Millisecond)
	}
	a.log.Error("s3 upload failed", "key", f.Key, "error", fmt.Sprint(lastErr))
	return lastErr
}
