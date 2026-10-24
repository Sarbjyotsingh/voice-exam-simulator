package exam

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"voice-engine/internal/contracts"
)

func TestScriptCachePreloadsAllB1ScenariosLocally(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))

	// Initialize with nil HTTP client to guarantee 0 network calls to S3
	cache := NewScriptCache(".", "", nil, log)

	logOutput := buf.String()
	expectedMsg := "Pre-loaded all B1 scenario scripts locally from catalog/ (0 network calls to S3)"
	if !strings.Contains(logOutput, expectedMsg) {
		t.Fatalf("expected startup log %q, got: %s", expectedMsg, logOutput)
	}

	ctx := context.Background()

	// Verify Teil 1 (Sample_01 .. Sample_05)
	for i := 1; i <= 5; i++ {
		for _, id := range []string{fmt.Sprintf("B1_S%02d", i), fmt.Sprintf("Sample_%02d", i), fmt.Sprintf("B1_Exam_Sample_%02d", i)} {
			entry, err := cache.Get(ctx, contracts.ExamTelc, contracts.LevelB1, id, 1)
			if err != nil || entry == nil || entry.Catalogue == nil {
				t.Fatalf("failed to get pre-loaded Teil 1 script for %s: %v", id, err)
			}
		}
	}

	// Verify Teil 2 (Sample_01 .. Sample_16)
	for i := 1; i <= 16; i++ {
		id := fmt.Sprintf("B1_S%02d", i)
		entry, err := cache.Get(ctx, contracts.ExamTelc, contracts.LevelB1, id, 2)
		if err != nil || entry == nil || entry.Catalogue == nil {
			t.Fatalf("failed to get pre-loaded Teil 2 script for %s: %v", id, err)
		}
	}

	// Verify Teil 3 (Sample_01 .. Sample_05)
	for i := 1; i <= 5; i++ {
		id := fmt.Sprintf("B1_S%02d", i)
		entry, err := cache.Get(ctx, contracts.ExamTelc, contracts.LevelB1, id, 3)
		if err != nil || entry == nil || entry.Catalogue == nil {
			t.Fatalf("failed to get pre-loaded Teil 3 script for %s: %v", id, err)
		}
	}
}
