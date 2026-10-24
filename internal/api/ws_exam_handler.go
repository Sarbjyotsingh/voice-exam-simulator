package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"voice-exam-simulator/config"
	"voice-exam-simulator/internal/contracts"
	"voice-exam-simulator/internal/engine"
	"voice-exam-simulator/internal/providers"
	"voice-exam-simulator/services"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type wsSafeConn struct {
	*websocket.Conn
	mu sync.Mutex
}

func (c *wsSafeConn) SendJSON(v interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.WriteJSON(v)
}

func (c *wsSafeConn) SendBytes(b []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.WriteMessage(websocket.BinaryMessage, b)
}

type registryTTSAdapter struct {
	reg *providers.ProviderRegistry
}

func (a registryTTSAdapter) Name() contracts.ProviderID {
	return contracts.ProviderElevenLabsTTS
}

func (a registryTTSAdapter) Synthesize(ctx context.Context, req providers.TTSRequest) ([]byte, contracts.CallUsage, error) {
	return a.reg.Synthesize(ctx, req)
}

// NewWSExamHandler handles WebSocket /exam (and /api/v1/turn) for both thin-client metadata payloads and legacy mobile payloads.
func NewWSExamHandler(
	cfg config.Config,
	registry *providers.ProviderRegistry,
	modes map[contracts.ModeID]engine.ModeHandler,
	arch engine.Archiver,
	ledger *engine.Ledger,
	log *slog.Logger,
) http.HandlerFunc {
	var activeConns int32
	streamer := engine.NewAudioStreamer(registryTTSAdapter{reg: registry}, log)

	return func(w http.ResponseWriter, r *http.Request) {
		current := atomic.AddInt32(&activeConns, 1)
		defer atomic.AddInt32(&activeConns, -1)

		if current > cfg.MaxConnections {
			http.Error(w, "too many connections", http.StatusServiceUnavailable)
			return
		}

		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Error("websocket upgrade failed", "error", err)
			return
		}
		defer c.Close()

		c.SetReadLimit(cfg.MaxAudioBytes)
		_ = c.SetReadDeadline(time.Now().Add(config.PongWait))
		c.SetPongHandler(func(string) error {
			_ = c.SetReadDeadline(time.Now().Add(config.PongWait))
			return nil
		})

		conn := &wsSafeConn{Conn: c}
		sessionID := uuid.New().String()
		sLog := log.With("session_id", sessionID)

		sLog.Info("WebSocket Connected")

		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		// Start keepalive
		go func() {
			ticker := time.NewTicker(config.PingPeriod)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					conn.mu.Lock()
					_ = conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(10*time.Second))
					conn.mu.Unlock()
				}
			}
		}()

		// Phase 1: Initialization
		_, initBytes, err := c.ReadMessage()
		if err != nil {
			sLog.Error("failed to read init data", "error", err)
			return
		}

		var initReq contracts.TurnInitRequest
		if err := json.Unmarshal(initBytes, &initReq); err != nil {
			sLog.Error("invalid initialization json", "error", err)
			_ = conn.SendJSON(map[string]string{"error": "Invalid initialization JSON"})
			return
		}

		sess := buildBaseSessionContext(initReq, sessionID, cfg.MaxHistoryMessages)
		modeHandler := modes[sess.Mode]
		if modeHandler == nil {
			modeHandler = modes[contracts.ModeExam]
		}
		if modeHandler != nil {
			if err := modeHandler.PrepareSession(ctx, initReq, sess); err != nil {
				sLog.Warn("mode PrepareSession warning", "error", err)
			}
		}
		resolveDefaultVoice(sess, initReq.VoiceID)

		sLog.Info("Session initialized",
			"mode", sess.Mode,
			"default_voice", sess.DefaultVoiceID,
			"attempt_id", sess.Attempt.AttemptID,
			"exam_id", sess.Attempt.ExamID,
			"has_catalogue", sess.Catalogue != nil,
		)
		_ = conn.SendJSON(map[string]string{
			"status":     string(contracts.WSStatusInitialized),
			"session_id": sess.SessionID,
		})

		// Phase 2: Interaction Loop
		for {
			msgType, audioBytes, err := c.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					sLog.Error("websocket read error", "error", err)
				}
				break
			}
			if msgType != websocket.BinaryMessage {
				continue
			}

			sLog.Info("Received audio", "bytes", len(audioBytes))

			turnCtx, usage := services.WithUsage(ctx)

			sttRes, _, err := registry.Transcribe(turnCtx, providers.STTRequest{
				Audio:    audioBytes,
				Language: sess.Language,
				Prompt:   config.GroqWhisperPrompt,
			})
			if err != nil {
				sLog.Error("transcription failed", "error", err)
				_ = conn.SendJSON(map[string]string{"status": string(contracts.WSStatusError), "error": "transcription failed"})
				continue
			}

			transcribedText := ""
			if sttRes != nil && !engine.IsHallucination(sess.Language, sttRes.Text) {
				transcribedText = sttRes.Text
			}

			if transcribedText == "" {
				res := ledger.RecordTurn(services.TurnInput{
					Key:     sess.Attempt,
					Status:  services.TurnEmptyTranscript,
					Usage:   usage.Snapshot(),
					AudioIn: audioBytes,
				})
				arch.Enqueue(res.Batch)
				_ = conn.SendJSON(map[string]string{
					"status":  string(contracts.WSStatusEmptyTranscription),
					"message": "I didn't hear anything clearly. Could you please repeat that?",
				})
				continue
			}

			sess.Messages = append(sess.Messages, contracts.ChatMessage{
				Role:    string(contracts.RoleUser),
				Content: transcribedText,
			})
			if len(sess.Messages) > cfg.MaxHistoryMessages+1 {
				sess.Messages = append(sess.Messages[:1], sess.Messages[2:]...)
			}

			_ = conn.SendJSON(map[string]string{
				"status":      string(contracts.WSStatusGenerating),
				"transcribed": transcribedText,
			})

			outcome, err := modeHandler.RouteTurn(turnCtx, sess, transcribedText)
			if err != nil {
				sLog.Error("turn routing / chat failed", "error", err)
				res := ledger.RecordTurn(services.TurnInput{
					Key:      sess.Attempt,
					Status:   services.TurnChatFailed,
					UserText: transcribedText,
					Usage:    usage.Snapshot(),
					AudioIn:  audioBytes,
				})
				arch.Enqueue(res.Batch)
				_ = conn.SendJSON(map[string]string{"status": string(contracts.WSStatusError), "error": "chat failed"})
				continue
			}

			allGeneratedAudio := streamer.StreamParts(turnCtx, sess, outcome.Parts, conn)

			sess.Messages = append(sess.Messages, contracts.ChatMessage{
				Role:    string(contracts.RoleAssistant),
				Content: outcome.RawJSON,
			})
			if len(sess.Messages) > cfg.MaxHistoryMessages+1 {
				sess.Messages = append(sess.Messages[:1], sess.Messages[2:]...)
			}

			res := ledger.RecordTurn(services.TurnInput{
				Key:       sess.Attempt,
				Status:    services.TurnOK,
				UserText:  transcribedText,
				LLMRaw:    outcome.RawJSON,
				AIParts:   outcome.Parts,
				ExamEnded: outcome.ExamEnded,
				Usage:     usage.Snapshot(),
				AudioIn:   audioBytes,
				AudioOut:  allGeneratedAudio,
			})
			arch.Enqueue(res.Batch)

			_ = conn.SendJSON(map[string]interface{}{
				"status":           string(contracts.WSStatusResponseComplete),
				"ai_text":          outcome.RawJSON,
				"parts":            outcome.Parts,
				"exam_ended":       outcome.ExamEnded,
				"attempt_id":       sess.Attempt.AttemptID,
				"turn_cost_usd":    res.TurnCostUSD,
				"attempt_cost_usd": res.AttemptCostUSD,
			})
		}
	}
}

func buildBaseSessionContext(initReq contracts.TurnInitRequest, sessionID string, maxHistory int) *engine.SessionContext {
	var hist []contracts.ChatMessage
	for _, rawMsg := range initReq.History {
		var m contracts.ChatMessage
		if err := json.Unmarshal(rawMsg, &m); err == nil && m.Role != string(contracts.RoleSystem) {
			hist = append(hist, m)
		}
	}
	if len(hist) > maxHistory {
		hist = hist[len(hist)-maxHistory:]
	}

	normMapping := make(map[string]string, len(initReq.VoiceMapping))
	for k, v := range initReq.VoiceMapping {
		normMapping[strings.ToLower(k)] = v
	}

	attemptID := strings.TrimSpace(initReq.AttemptID)
	if attemptID == "" {
		attemptID = "legacy_" + sessionID
	}

	return &engine.SessionContext{
		SessionID:        sessionID,
		Mode:             contracts.NormalizeMode(initReq.Mode),
		Language:         contracts.NormalizeLanguage(initReq.Language),
		ExamProvider:     contracts.NormalizeExamProvider(initReq.ExamProvider),
		ExamLevel:        contracts.NormalizeCEFRLevel(initReq.ExamLevel),
		TeilNumber:       initReq.TeilNumber,
		ExamID:           initReq.ExamID,
		ScenarioID:       initReq.ScenarioID,
		PrimarySpeaker:   initReq.PrimarySpeaker,
		SecondarySpeaker: initReq.SecondarySpeaker,
		VoiceMapping:     normMapping,
		Messages:         hist,
		Attempt: services.AttemptKey{
			UserID:     initReq.UserID,
			ExamID:     initReq.ExamID,
			ExamLevel:  initReq.ExamLevel,
			TeilNumber: initReq.TeilNumber,
			AttemptID:  attemptID,
		},
	}
}

func resolveDefaultVoice(s *engine.SessionContext, explicitVoiceID string) {
	defVoice := explicitVoiceID
	primary := strings.ToLower(s.PrimarySpeaker)
	secondary := strings.ToLower(s.SecondarySpeaker)

	if defVoice == "" && primary != "" {
		if v, ok := s.VoiceMapping[primary]; ok {
			defVoice = v
		}
	}
	if defVoice == "" {
		for _, key := range []string{"participant", "candidate", "partner", "primary", "assistant", "default"} {
			if v, ok := s.VoiceMapping[key]; ok {
				defVoice = v
				break
			}
		}
	}
	if defVoice == "" && len(s.VoiceMapping) > 0 {
		var nonExaminerKeys, allKeys []string
		for k := range s.VoiceMapping {
			allKeys = append(allKeys, k)
			if (secondary == "" || k != secondary) && !strings.Contains(k, "examiner") && !strings.Contains(k, "prüfer") {
				nonExaminerKeys = append(nonExaminerKeys, k)
			}
		}
		sort.Strings(nonExaminerKeys)
		sort.Strings(allKeys)
		if len(nonExaminerKeys) > 0 {
			defVoice = s.VoiceMapping[nonExaminerKeys[0]]
		} else if len(allKeys) > 0 {
			defVoice = s.VoiceMapping[allKeys[0]]
		}
	}
	s.DefaultVoiceID = defVoice
}
