// Package iohelp provides shared JSON import/export helpers for all data stores.
package iohelp

import (
	"encoding/json"
	"io"
	"time"
)

// Export wraps store items with metadata for portable JSON export.
type Export struct {
	Version    string      `json:"version"`
	Type       string      `json:"type"`
	ExportedAt int64       `json:"exported_at"`
	Count      int         `json:"count"`
	Items      interface{} `json:"items"`
}

// NewExport creates a new Export wrapper.
func NewExport(typ string, items interface{}, count int) *Export {
	return &Export{
		Version:    "1.0",
		Type:       typ,
		ExportedAt: time.Now().Unix(),
		Count:      count,
		Items:      items,
	}
}

// ExportJSON writes items as a wrapped JSON export to w.
func ExportJSON(w io.Writer, typ string, items interface{}, count int) error {
	exp := NewExport(typ, items, count)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(exp)
}

// ImportJSON reads a wrapped JSON export from r, returning the raw items.
// The caller is responsible for type-asserting the items slice.
func ImportJSON(r io.Reader) (*Export, error) {
	var exp Export
	if err := json.NewDecoder(r).Decode(&exp); err != nil {
		return nil, err
	}
	return &exp, nil
}
