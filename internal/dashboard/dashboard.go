// Package dashboard renders the admin / ops console: it composes the digest
// (breadth + signal sections) with warehouse/ingestion stats and the Sentinel
// Tier-0 data-quality watch into one live HTML view for `scanner serve`.
package dashboard

import (
	"html/template"
	"io"
	"math"
	"strconv"

	"cetus-marketdata-scanner/internal/digest"
	"cetus-marketdata-scanner/internal/sentinel"
	"cetus-marketdata-scanner/internal/store"
	"cetus-marketdata-scanner/internal/user"
)

// Model is everything the two pages render: the user dashboard (/) shows breadth +
// the acting user's signal studies; the admin console (/admin) adds ops state,
// the data-quality watch, and the user registry.
type Model struct {
	Acting      user.User
	SessionAuth bool // true in built-in login mode → show Sign-out; false behind a proxy
	Stats       store.OpsStats
	DBSizeBytes int64
	ScanMillis  int64
	Digest      digest.Digest
	Flags       []sentinel.Flag
	Suspect     int
	Watch       int
	Users       []user.User
}

// IndexHTML renders the user-facing dashboard (route "/").
func (m Model) IndexHTML(w io.Writer) error { return tmpl.ExecuteTemplate(w, "index", m) }

// AdminHTML renders the admin ops console (route "/admin").
func (m Model) AdminHTML(w io.Writer) error { return tmpl.ExecuteTemplate(w, "admin", m) }

// Login is the sign-in page model.
type Login struct {
	Error string
	Users []user.User
}

// HTML renders the sign-in page.
func (l Login) HTML(w io.Writer) error { return tmpl.ExecuteTemplate(w, "login", l) }

// --- template helpers ---

func money(v float64) string {
	if math.IsNaN(v) {
		return "—"
	}
	switch {
	case v >= 1e9:
		return "$" + strconv.FormatFloat(v/1e9, 'f', 1, 64) + "B"
	case v >= 1e6:
		return "$" + strconv.FormatFloat(v/1e6, 'f', 1, 64) + "M"
	case v >= 1e3:
		return "$" + strconv.FormatFloat(v/1e3, 'f', 1, 64) + "K"
	default:
		return "$" + strconv.FormatFloat(v, 'f', 0, 64)
	}
}

func retpct(v float64) string {
	if math.IsNaN(v) {
		return "—"
	}
	s := strconv.FormatFloat(v*100, 'f', 0, 64)
	if v >= 0 {
		return "+" + s + "%"
	}
	return s + "%"
}

func num1(v float64) string {
	if math.IsNaN(v) {
		return "—"
	}
	return strconv.FormatFloat(v, 'f', 1, 64)
}

func num2(v float64) string {
	if math.IsNaN(v) {
		return "—"
	}
	return strconv.FormatFloat(v, 'f', 2, 64)
}

func ratio(v float64) string {
	if math.IsNaN(v) {
		return "—"
	}
	return strconv.FormatFloat(v, 'f', 2, 64) + "x"
}

func humanInt(n int64) string {
	s := strconv.FormatInt(n, 10)
	if n < 0 {
		return s
	}
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	return string(out)
}

func dbSize(bytes int64) string {
	switch {
	case bytes >= 1<<30:
		return strconv.FormatFloat(float64(bytes)/(1<<30), 'f', 1, 64) + " GB"
	case bytes >= 1<<20:
		return strconv.FormatInt(bytes>>20, 10) + " MB"
	default:
		return strconv.FormatInt(bytes>>10, 10) + " KB"
	}
}

func ms(millis int64) string {
	return strconv.FormatFloat(float64(millis)/1000, 'f', 2, 64) + " s"
}

var funcs = template.FuncMap{
	"money": money, "retpct": retpct, "num1": num1, "num2": num2,
	"ratio": ratio, "humanInt": humanInt, "dbSize": dbSize, "ms": ms,
	"int64": func(n int) int64 { return int64(n) },
	"gt0":   func(v float64) bool { return v > 0.05 },
	"lt0":   func(v float64) bool { return v < -0.05 },
	"upper": func(s sentinel.Severity) string {
		b := []byte(string(s))
		for i := range b {
			if b[i] >= 'a' && b[i] <= 'z' {
				b[i] -= 32
			}
		}
		return string(b)
	},
}

var tmpl = template.Must(template.New("dash").Funcs(funcs).Parse(stylesSrc + indexSrc + adminSrc + loginSrc))
