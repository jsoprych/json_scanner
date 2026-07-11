package dblog

import (
	"database/sql"
	"log/slog"
	"os"
	"testing"

	_ "modernc.org/sqlite"
)

func testDB(t *testing.T) *DB {
	t.Helper()
	raw, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY, val TEXT)"); err != nil {
		t.Fatal(err)
	}
	return New(raw, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
}

func TestNew(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	if db == nil {
		t.Fatal("New returned nil")
	}
	if db.DB() == nil {
		t.Fatal("DB() returned nil")
	}
}

func TestExec(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	_, err := db.Exec("INSERT INTO test (id, val) VALUES (?, ?)", 1, "hello")
	if err != nil {
		t.Fatal(err)
	}
	var val string
	db.QueryRow("SELECT val FROM test WHERE id = ?", 1).Scan(&val)
	if val != "hello" {
		t.Errorf("got %q want %q", val, "hello")
	}
}

func TestQuery(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	db.Exec("INSERT INTO test (id, val) VALUES (1, 'a'), (2, 'b')")
	rows, err := db.Query("SELECT val FROM test ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var vals []string
	for rows.Next() {
		var v string
		rows.Scan(&v)
		vals = append(vals, v)
	}
	if len(vals) != 2 || vals[0] != "a" || vals[1] != "b" {
		t.Errorf("got %v want [a b]", vals)
	}
}

func TestQueryRow(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	db.Exec("INSERT INTO test (id, val) VALUES (1, 'x')")
	var val string
	err := db.QueryRow("SELECT val FROM test WHERE id = ?", 1).Scan(&val)
	if err != nil {
		t.Fatal(err)
	}
	if val != "x" {
		t.Errorf("got %q want %q", val, "x")
	}
}

func TestBegin(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec("INSERT INTO test (id, val) VALUES (1, 'tx')")
	if err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	tx.Commit()
	var val string
	db.QueryRow("SELECT val FROM test WHERE id = 1").Scan(&val)
	if val != "tx" {
		t.Errorf("got %q want %q", val, "tx")
	}
}

func TestExecError(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	_, err := db.Exec("INSERT INTO nonexistent VALUES (1)")
	if err == nil {
		t.Error("expected error")
	}
}

func TestQueryError(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	_, err := db.Query("SELECT * FROM nonexistent")
	if err == nil {
		t.Error("expected error")
	}
}

func TestClose(t *testing.T) {
	db := testDB(t)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
}
