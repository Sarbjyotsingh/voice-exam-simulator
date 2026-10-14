package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"ai_exam_simulator/config"
	"ai_exam_simulator/services"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins like Python FastAPI
	},
}

type safeConn struct {
	*websocket.Conn
	mu sync.Mutex
}

func (c *safeConn) sendJSON(v interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.WriteJSON(v)
}

func (c *safeConn) sendBytes(b []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.WriteMessage(websocket.BinaryMessage, b)
}

func NewExam(
	cfg config.Config,
	stt Transcriber,
	chat Chatter,
	tts Synthesizer,
	arch services.Archiver,
	log *slog.Logger,
) http.HandlerFunc {
	var activeConns int32

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

		conn := &safeConn{Conn: c}
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
		_, initDataStr, err := c.ReadMessage()
		if err != nil {
			sLog.Error("failed to read init data", "error", err)
			return
		}

		session, err := parseInit(initDataStr, sessionID, cfg.MaxHistoryMessages)
		if err != nil {
			sLog.Error("invalid initialization json", "error", err)
			_ = conn.sendJSON(map[string]string{"error": "Invalid initialization JSON"})
			return
		}

		sLog.Info("Session initialized", "default_voice", session.DefaultVoiceID)
		_ = conn.sendJSON(map[string]string{
			"status":     config.StatusInitialized,
			"session_id": session.ID,
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

			transcribedText, err := stt.Transcribe(ctx, audioBytes)
			if err != nil {
				sLog.Error("transcription failed", "error", err)
				_ = conn.sendJSON(map[string]string{"status": config.StatusError, "error": "transcription failed"})
				continue
			}

			if transcribedText == "" {
				_ = conn.sendJSON(map[string]string{
					"status":  config.StatusEmptyTranscription,
					"message": "I didn't hear anything clearly. Could you please repeat that?",
				})
				continue
			}

			session.Messages = append(session.Messages, services.Message{Role: "user", Content: transcribedText})
			if len(session.Messages) > cfg.MaxHistoryMessages+1 {
				session.Messages = append(session.Messages[:1], session.Messages[2:]...) // Keep sys prompt
			}

			_ = conn.sendJSON(map[string]string{
				"status":      config.StatusGeneratingResponse,
				"transcribed": transcribedText,
			})

			llmResp, err := chat.Chat(ctx, session.Messages)
			if err != nil {
				sLog.Error("chat failed", "error", err)
				_ = conn.sendJSON(map[string]string{"status": config.StatusError, "error": "chat failed"})
				continue
			}

			var responseData struct {
				Parts     []map[string]interface{} `json:"parts"`
				ExamEnded bool                     `json:"exam_ended"`
			}
			if err := json.Unmarshal([]byte(llmResp), &responseData); err != nil {
				sLog.Error("failed to parse LLM JSON", "error", err)
				responseData.Parts = []map[string]interface{}{}
			}

			// Generate audio for all parts concurrently
			type TTSResult struct {
				Index int
				Bytes []byte
				Err   error
			}
			results := make(chan TTSResult, len(responseData.Parts))
			var wg sync.WaitGroup

			for i, p := range responseData.Parts {
				if text, ok := p["text"].(string); ok && text != "" {
					wg.Add(1)
					go func(idx int, t string, vID string) {
						defer wg.Done()
						b, e := tts.Synthesize(ctx, t, vID)
						results <- TTSResult{Index: idx, Bytes: b, Err: e}
					}(i, text, session.voiceFor(p))
				}
			}

			go func() {
				wg.Wait()
				close(results)
			}()

			// Collect results
			audioParts := make(map[int][]byte)
			for r := range results {
				if r.Err != nil {
					sLog.Error("TTS failed", "error", r.Err)
				} else {
					audioParts[r.Index] = r.Bytes
				}
			}

			// Send back in order
			var allGeneratedAudio []byte
			for i, p := range responseData.Parts {
				audioID, _ := p["audio_id"].(string)
				if audioID == "" {
					audioID, _ = p["audioId"].(string)
				}

				if audioID != "" {
					_ = conn.sendJSON(map[string]string{
						"status":   config.StatusPlayAudioID,
						"audio_id": audioID,
					})
				} else if b, ok := audioParts[i]; ok {
					_ = conn.sendBytes(b)
					allGeneratedAudio = append(allGeneratedAudio, b...)
				}
			}

			session.Messages = append(session.Messages, services.Message{Role: "assistant", Content: llmResp})
			if len(session.Messages) > cfg.MaxHistoryMessages+1 {
				session.Messages = append(session.Messages[:1], session.Messages[2:]...) // Keep sys prompt
			}

			_ = conn.sendJSON(map[string]interface{}{
				"status":     config.StatusResponseComplete,
				"ai_text":    llmResp,
				"parts":      responseData.Parts,
				"exam_ended": responseData.ExamEnded,
			})

			arch.Enqueue(services.ArchiveJob{
				SessionID: session.ID,
				Metadata: services.SessionMetadata{
					Transcription: transcribedText,
					LLMResponse:   llmResp,
				},
				AudioOut: allGeneratedAudio,
				AudioIn:  audioBytes,
			})
		}
	}
}
