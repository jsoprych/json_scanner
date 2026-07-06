// Package digest assembles the daily post-close signal digest from a scanned
// snapshot and renders it as responsive HTML (email-safe), plaintext, and JSON.
// It is pure formatting — no I/O beyond writing to the caller's io.Writer, no deps
// beyond the stdlib. See docs/PHASE1_MVP.md.
package digest

import (
	"encoding/json"
	"io"
	"math"
	"strconv"
	"time"

	htmltemplate "html/template"
	texttemplate "text/template"

	"cetus-marketdata-scanner/internal/screen"
)

// Section is one preset's ranked result within the digest.
type Section struct {
	Key   string              `json:"key"`
	Title string              `json:"title"`
	Emoji string              `json:"emoji"`
	Rows  []screen.SnapshotRow `json:"rows"`
}

// Digest is the full rendered-ready daily report.
type Digest struct {
	DateLabel      string          `json:"date"`
	GeneratedAt    time.Time       `json:"generated_at"`
	SymbolsScanned int             `json:"symbols_scanned"`
	Breadth        screen.Breadth  `json:"breadth"`
	Sections       []Section       `json:"sections"`
}

// Build assembles a Digest for the given trading day. presets are applied to rows
// (already liquidity-filtered by the caller) to produce the sections.
func Build(day time.Time, scanned int, rows []screen.SnapshotRow, presets []screen.Preset) Digest {
	sections := make([]Section, 0, len(presets))
	for _, p := range presets {
		sections = append(sections, Section{
			Key:   p.Key,
			Title: p.Title,
			Emoji: p.Emoji,
			Rows:  p.Run(rows),
		})
	}
	return Digest{
		DateLabel:      day.Format("Monday, January 2, 2006"),
		GeneratedAt:    time.Now().UTC(),
		SymbolsScanned: scanned,
		Breadth:        screen.ComputeBreadth(rows),
		Sections:       sections,
	}
}

// JSON writes the digest as indented JSON (the API / mail-layer payload).
func (d Digest) JSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(d)
}

// HTML renders the responsive email/web digest.
func (d Digest) HTML(w io.Writer) error {
	return htmlTmpl.Execute(w, d)
}

// Text renders the plaintext fallback.
func (d Digest) Text(w io.Writer) error {
	return textTmpl.Execute(w, d)
}

// --- formatting helpers (shared by both templates) ---

func f2(v float64) string {
	if math.IsNaN(v) {
		return "—"
	}
	return trimNum(v, 2)
}

// pct1 formats a fraction (0.23 → "23.0%").
func pct1(v float64) string {
	if math.IsNaN(v) {
		return "—"
	}
	return trimNum(v*100, 1) + "%"
}

// pp1 formats a percentage-point value already in %.
func pp1(v float64) string {
	if math.IsNaN(v) {
		return "—"
	}
	return trimNum(v, 1)
}

// arrow renders a signed day-over-day delta (in pp) with direction glyph.
func arrow(delta float64) string {
	switch {
	case math.IsNaN(delta):
		return ""
	case delta > 0.05:
		return "▲ +" + pp1(delta) + " pp"
	case delta < -0.05:
		return "▼ " + pp1(delta) + " pp"
	default:
		return "▬ flat"
	}
}

// money humanizes a dollar figure (1.2B / 340.0M / 5.0K).
func money(v float64) string {
	if math.IsNaN(v) {
		return "—"
	}
	switch {
	case v >= 1e9:
		return "$" + trimNum(v/1e9, 1) + "B"
	case v >= 1e6:
		return "$" + trimNum(v/1e6, 1) + "M"
	case v >= 1e3:
		return "$" + trimNum(v/1e3, 1) + "K"
	default:
		return "$" + trimNum(v, 0)
	}
}

// breadthMood is a one-line human read of the tape.
func breadthMood(b screen.Breadth) string {
	switch d := b.DeltaAbove200(); {
	case d > 1:
		return "Breadth broadening."
	case d < -1:
		return "Breadth narrowing."
	default:
		return "Breadth steady."
	}
}

func trimNum(v float64, dec int) string {
	return strconv.FormatFloat(v, 'f', dec, 64)
}

var funcs = map[string]any{
	"f2": f2, "pct1": pct1, "pp1": pp1, "arrow": arrow, "money": money, "mood": breadthMood,
}

var htmlTmpl = htmltemplate.Must(htmltemplate.New("digest").Funcs(funcs).Parse(htmlSrc))
var textTmpl = texttemplate.Must(texttemplate.New("digest").Funcs(funcs).Parse(textSrc))
