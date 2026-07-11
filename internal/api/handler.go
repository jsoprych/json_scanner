package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"cetus-marketdata-scanner/internal/alert"
	"cetus-marketdata-scanner/internal/authjwt"
	"cetus-marketdata-scanner/internal/backtest"
	"cetus-marketdata-scanner/internal/groups"
	"cetus-marketdata-scanner/internal/permissions"
	"cetus-marketdata-scanner/internal/results"
	"cetus-marketdata-scanner/internal/roles"
	"cetus-marketdata-scanner/internal/snapshot"
	"cetus-marketdata-scanner/internal/store"
	"cetus-marketdata-scanner/internal/study"
	"cetus-marketdata-scanner/internal/throttle"
	"cetus-marketdata-scanner/internal/user"
)

// Handler provides REST API endpoints.
type Handler struct {
	snap         *snapshot.DB
	studies      *study.Store
	store        *store.Store
	users        *user.Store
	subs         *study.SubscriptionStore
	groups       *groups.Store
	results      *results.Store
	roles        *roles.Store
	permChecker  *permissions.Checker
	throttler    *throttle.Throttler
	detector     *alert.Detector
	backtest     *backtest.Engine
	signer       *authjwt.Signer
	verifier     interface {
		Verify(string) (string, error)
	}
	validateSession func(cookie string) (userID string, isAdmin bool, valid bool)
	log   *slog.Logger
	start time.Time
}

// NewHandler creates a minimal API handler (health + features only).
func NewHandler(snap *snapshot.DB, log *slog.Logger) *Handler {
	return &Handler{snap: snap, log: log, start: time.Now()}
}

// NewHandlerFull creates a handler with all dependencies.
func NewHandlerFull(
	snap *snapshot.DB,
	studies *study.Store,
	st *store.Store,
	users *user.Store,
	subs *study.SubscriptionStore,
	groups *groups.Store,
	results *results.Store,
	roles *roles.Store,
	permChecker *permissions.Checker,
	throttler *throttle.Throttler,
	detector *alert.Detector,
	backtest *backtest.Engine,
	signer *authjwt.Signer,
	verifier interface{ Verify(string) (string, error) },
	validateSession func(cookie string) (userID string, isAdmin bool, valid bool),
	log *slog.Logger,
) *Handler {
	return &Handler{
		snap:            snap,
		studies:         studies,
		store:           st,
		users:           users,
		subs:            subs,
		groups:          groups,
		results:         results,
		roles:           roles,
		permChecker:     permChecker,
		throttler:       throttler,
		detector:        detector,
		backtest:        backtest,
		signer:          signer,
		verifier:        verifier,
		validateSession: validateSession,
		log:             log,
		start:           time.Now(),
	}
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// ErrorResponse is the error response format.
type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, ErrorResponse{Error: message, Code: code})
}
