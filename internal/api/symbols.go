package api

import (
	"net/http"
	"strconv"
	"time"

	"cetus-marketdata-scanner/internal/model"
)

// UniverseResponse is the response for /universe.
type UniverseResponse struct {
	Symbols []string `json:"symbols"`
	Count   int      `json:"count"`
}

// Universe returns the list of scannable symbols.
func (h *Handler) Universe(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_STORE", "warehouse not connected")
		return
	}
	symbols, err := h.store.Universe(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "UNIVERSE_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, UniverseResponse{Symbols: symbols, Count: len(symbols)})
}

// SymbolResponse is the response for /symbols/{symbol}.
type SymbolResponse struct {
	Symbol string      `json:"symbol"`
	Bars   []model.Bar `json:"bars"`
	Count  int         `json:"count"`
}

// Symbol returns recent bars for a symbol.
func (h *Handler) Symbol(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_STORE", "warehouse not connected")
		return
	}
	symbol := r.PathValue("symbol")
	if symbol == "" {
		writeError(w, http.StatusBadRequest, "MISSING_SYMBOL", "symbol required")
		return
	}

	since := time.Now().UTC().AddDate(0, 0, -120).Unix()
	if s := r.URL.Query().Get("since"); s != "" {
		if days, err := strconv.Atoi(s); err == nil {
			since = time.Now().UTC().AddDate(0, 0, -days).Unix()
		}
	}

	bars, err := h.store.LoadAdjustedBars(r.Context(), symbol, since)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "LOAD_FAILED", err.Error())
		return
	}
	if len(bars) == 0 {
		writeError(w, http.StatusNotFound, "NO_DATA", "no bars found for symbol")
		return
	}
	writeJSON(w, http.StatusOK, SymbolResponse{Symbol: symbol, Bars: bars, Count: len(bars)})
}
