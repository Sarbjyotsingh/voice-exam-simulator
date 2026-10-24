package contracts

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var unsafeIDChars = regexp.MustCompile(`[^A-Za-z0-9._-]`)
var sampleNumRe = regexp.MustCompile(`(?i)(?:S|Sample_?)(\d+)`)

// SanitizeID makes a client-supplied identifier safe to use inside an S3 key or file path.
func SanitizeID(s, fallback string) string {
	s = strings.Trim(unsafeIDChars.ReplaceAllString(strings.TrimSpace(s), "_"), "._")
	if s == "" {
		return fallback
	}
	if len(s) > 128 {
		s = s[:128]
	}
	return s
}

// SpecKey builds a deterministic lookup key for an exam Teil spec: e.g. "exam:de:telc:B1:2".
func SpecKey(mode ModeID, lang LanguageCode, provider ExamProvider, level CEFRLevel, teil int) string {
	return fmt.Sprintf("%s:%s:%s:%s:%d",
		NormalizeMode(string(mode)),
		NormalizeLanguage(string(lang)),
		NormalizeExamProvider(string(provider)),
		NormalizeCEFRLevel(string(level)),
		teil,
	)
}

// ExamFolderFromID maps an exam_id like "B1_S01" or "B1_Exam_Sample_01" to the canonical folder "B1_Exam_Sample_01".
func ExamFolderFromID(level CEFRLevel, examID string) string {
	lvl := string(NormalizeCEFRLevel(string(level)))
	trimmed := strings.TrimSpace(examID)
	if strings.Contains(trimmed, "_Exam_Sample_") {
		return trimmed
	}
	if m := sampleNumRe.FindStringSubmatch(trimmed); len(m) == 2 {
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
			return fmt.Sprintf("%s_Exam_Sample_%02d", lvl, n)
		}
	}
	if trimmed == "" {
		return fmt.Sprintf("%s_Exam_Sample_01", lvl)
	}
	return trimmed
}

// SampleFolderFromID maps an exam_id like "B1_S01", "B1_Exam_Sample_01", or "Sample_01" to the catalog scenario folder "sample_01".
func SampleFolderFromID(examID string) string {
	trimmed := strings.TrimSpace(examID)
	if m := sampleNumRe.FindStringSubmatch(trimmed); len(m) == 2 {
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
			return fmt.Sprintf("sample_%02d", n)
		}
	}
	if trimmed == "" {
		return "sample_01"
	}
	return strings.ToLower(trimmed)
}

// LocalScriptCandidates returns candidate local file paths for a (level, examID, teil) script.json.
func LocalScriptCandidates(baseDir string, provider ExamProvider, level CEFRLevel, examID string, teil int) []string {
	folder := ExamFolderFromID(level, examID)
	sampleFolder := SampleFolderFromID(examID)
	teilDir := fmt.Sprintf("Teil_%d", teil)
	catalogTeilDir := fmt.Sprintf("teil_%d", teil)
	provLower := strings.ToLower(string(NormalizeExamProvider(string(provider))))
	lvlLower := strings.ToLower(string(NormalizeCEFRLevel(string(level))))
	examSubDir := fmt.Sprintf("%s_%s", provLower, lvlLower)

	var candidates []string
	roots := []string{baseDir}
	if baseDir == "" || baseDir == "." {
		roots = []string{".", "..", "../..", "../../.."}
	}
	for _, r := range roots {
		candidates = append(candidates,
			filepath.Join(r, "catalog", "exams", "de", examSubDir, "scenarios", sampleFolder, catalogTeilDir, "script.json"),
			filepath.Join(r, "exams", "de", examSubDir, "scenarios", sampleFolder, catalogTeilDir, "script.json"),
			filepath.Join(r, "exam_generator", "output_s3", folder, teilDir, "script.json"),
			filepath.Join(r, "exam_generator", "upload_package", provLower, lvlLower, "speaking", folder, teilDir, "script.json"),
		)
	}
	return candidates
}

// ContentScriptURL builds the S3/CDN URL for a script.json file.
func ContentScriptURL(baseURL string, provider ExamProvider, level CEFRLevel, examID string, teil int) string {
	baseURL = strings.TrimRight(baseURL, "/")
	folder := ExamFolderFromID(level, examID)
	provLower := strings.ToLower(string(NormalizeExamProvider(string(provider))))
	lvlLower := strings.ToLower(string(NormalizeCEFRLevel(string(level))))
	return fmt.Sprintf("%s/%s/%s/speaking/%s/Teil_%d/script.json", baseURL, provLower, lvlLower, folder, teil)
}

// ContentManifestURL builds the S3/CDN URL for an exam level manifest.
func ContentManifestURL(baseURL string, lang LanguageCode, provider ExamProvider, level CEFRLevel) string {
	baseURL = strings.TrimRight(baseURL, "/")
	provLower := strings.ToLower(string(NormalizeExamProvider(string(provider))))
	lvlLower := strings.ToLower(string(NormalizeCEFRLevel(string(level))))
	_ = lang
	return fmt.Sprintf("%s/%s/%s/speaking/manifest.json", baseURL, provLower, lvlLower)
}

// AuditExamKey returns the ledger exam key (e.g. "B1_B1_S01_teil2").
func AuditExamKey(level, examID string, teil int) string {
	return fmt.Sprintf("%s_%s_teil%d", SanitizeID(level, "unknown"), SanitizeID(examID, "unknown"), teil)
}

// AuditUserSummaryKey returns "users/<userID>/user_summary.json".
func AuditUserSummaryKey(userID string) string {
	return fmt.Sprintf("users/%s/user_summary.json", SanitizeID(userID, "anonymous"))
}

// AuditExamSummaryKey returns "users/<userID>/exams/<examKey>/exam_summary.json".
func AuditExamSummaryKey(userID, examKey string) string {
	return fmt.Sprintf("users/%s/exams/%s/exam_summary.json", SanitizeID(userID, "anonymous"), examKey)
}

// AuditAttemptDir returns "users/<userID>/exams/<examKey>/attempts/<attemptID>".
func AuditAttemptDir(userID, examKey, attemptID string) string {
	return fmt.Sprintf("users/%s/exams/%s/attempts/%s", SanitizeID(userID, "anonymous"), examKey, SanitizeID(attemptID, "unknown"))
}
