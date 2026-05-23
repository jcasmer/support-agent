package http

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/jcasmer/support-agent/internal/adapters/in/http/middleware"
	"github.com/jcasmer/support-agent/internal/core"
)

// Runner is the port this adapter depends on.
// The orchestrator implements this.
type Runner interface {
	Run(ctx context.Context, sessionID, query string) (core.Result, error)
}

// Handler handles incoming support HTTP requests
type Handler struct {
	runner Runner
}

// New creates a new Handler
func New(runner Runner) *Handler {
	return &Handler{runner: runner}
}

// supportRequest is the expected JSON body
type supportRequest struct {
	SessionID string `json:"session_id"`
	Query     string `json:"query"`
}

// supportResponse is the JSON response shape
type supportResponse struct {
	SessionID  string `json:"session_id"`
	Response   string `json:"response"`
	Category   string `json:"category"`
	Resolved   bool   `json:"resolved"`
	TicketID   string `json:"ticket_id,omitempty"`
	TokensUsed int    `json:"tokens_used"`
	DurationMs int64  `json:"duration_ms"`
	RequestID  string `json:"request_id"`
}

// errorResponse is the standard error envelope
type errorResponse struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

// Handle processes a support request
func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.FromContext(r.Context())

	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only POST is accepted", requestID)
		return
	}

	var req supportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON", requestID)
		return
	}
	defer r.Body.Close()

	if req.Query == "" {
		writeError(w, http.StatusBadRequest, "missing_field", "query is required", requestID)
		return
	}

	if req.SessionID == "" {
		req.SessionID = requestID
	}

	slog.Info("support request received",
		"request_id", requestID,
		"session_id", req.SessionID,
		"query_length", len(req.Query),
	)

	result, err := h.runner.Run(r.Context(), req.SessionID, req.Query)
	if err != nil {
		slog.Error("orchestration failed",
			"request_id", requestID,
			"session_id", req.SessionID,
			"error", err,
		)
		writeError(w, http.StatusInternalServerError, "orchestration_error", "failed to process support request", requestID)
		return
	}

	writeJSON(w, http.StatusOK, supportResponse{
		SessionID:  result.SessionID,
		Response:   result.Response,
		Category:   result.Category,
		Resolved:   result.Resolved,
		TicketID:   result.TicketID,
		TokensUsed: result.TokensUsed,
		DurationMs: result.Duration.Milliseconds(),
		RequestID:  requestID,
	})
}

// writeJSON serializes v as JSON and writes it to w
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("failed to write response", "error", err)
	}
}

// writeError writes a standard error envelope
func writeError(w http.ResponseWriter, status int, code, message, requestID string) {
	writeJSON(w, status, errorResponse{
		Code:      code,
		Message:   message,
		RequestID: requestID,
	})
}