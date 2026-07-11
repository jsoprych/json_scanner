package iohelp

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

type testItem struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func TestNewExport(t *testing.T) {
	items := []testItem{{1, "a"}, {2, "b"}}
	exp := NewExport("tests", items, 2)
	if exp.Version != "1.0" {
		t.Errorf("version: %s", exp.Version)
	}
	if exp.Type != "tests" {
		t.Errorf("type: %s", exp.Type)
	}
	if exp.Count != 2 {
		t.Errorf("count: %d", exp.Count)
	}
	if exp.ExportedAt == 0 {
		t.Error("exported_at is zero")
	}
}

func TestExportJSON(t *testing.T) {
	items := []testItem{{1, "hello"}, {2, "world"}}
	var buf bytes.Buffer
	err := ExportJSON(&buf, "tests", items, 2)
	if err != nil {
		t.Fatal(err)
	}
	var exp Export
	if err := json.Unmarshal(buf.Bytes(), &exp); err != nil {
		t.Fatal(err)
	}
	if exp.Type != "tests" || exp.Count != 2 {
		t.Errorf("type=%s count=%d", exp.Type, exp.Count)
	}
}

func TestImportJSON(t *testing.T) {
	items := []testItem{{1, "foo"}, {2, "bar"}}
	var buf bytes.Buffer
	ExportJSON(&buf, "tests", items, 2)

	exp, err := ImportJSON(strings.NewReader(buf.String()))
	if err != nil {
		t.Fatal(err)
	}
	if exp.Type != "tests" {
		t.Errorf("type=%s", exp.Type)
	}
	if exp.Count != 2 {
		t.Errorf("count=%d", exp.Count)
	}
	// Marshal items back to verify
	data, _ := json.Marshal(exp.Items)
	var decoded []testItem
	json.Unmarshal(data, &decoded)
	if len(decoded) != 2 || decoded[0].Name != "foo" {
		t.Errorf("got %+v", decoded)
	}
}

func TestImportJSONBadInput(t *testing.T) {
	_, err := ImportJSON(strings.NewReader("not json"))
	if err == nil {
		t.Error("expected error for bad input")
	}
}
