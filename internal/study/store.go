package study

import (
	"cetus-marketdata-scanner/internal/dblog"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"cetus-marketdata-scanner/internal/iohelp"
	"cetus-marketdata-scanner/internal/user"
)

// Store is a mutable set of studies backed by SQLite.
// JSONL files are for import/export only — NOT synced on writes.
type Store struct {
	db    *dblog.DB
	mu    sync.Mutex
	all   []Study
	byKey map[string]Study
}

// OpenStore loads studies from JSONL path (legacy, no DB).
func OpenStore(path string) (*Store, error) {
	s := &Store{byKey: map[string]Study{}}
	if err := s.importJSONLFile(path); err != nil {
		return nil, err
	}
	return s, nil
}

// OpenStoreWithDB opens a SQLite-backed study store.
func OpenStoreWithDB(db *dblog.DB, jsonlPath string) (*Store, error) {
	s := &Store{db: db, byKey: map[string]Study{}}

	if err := s.loadFromSQL(); err != nil {
		return nil, fmt.Errorf("load studies from SQLite: %w", err)
	}

	// One-time migration: if SQLite is empty, import from JSONL
	if len(s.all) == 0 && jsonlPath != "" {
		if err := s.importJSONLFile(jsonlPath); err != nil {
			return nil, err
		}
		for _, st := range s.all {
			if err := s.saveToSQL(st); err != nil {
				return nil, err
			}
		}
	}
	return s, nil
}

func (s *Store) importJSONLFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()
	all, err := LoadJSONL(f)
	if err != nil {
		return err
	}
	for _, st := range all {
		s.all = append(s.all, st)
		s.byKey[st.Key] = st
	}
	return nil
}

func (s *Store) loadFromSQL() error {
	rows, err := s.db.Query("SELECT key, owner, visibility, group_name, tier, title, emoji, where_clause, order_by, limit_num FROM studies ORDER BY key")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var st Study
		if err := rows.Scan(&st.Key, &st.Owner, &st.Visibility, &st.Group, &st.Tier, &st.Title, &st.Emoji, &st.Where, &st.OrderBy, &st.Limit); err != nil {
			return err
		}
		s.all = append(s.all, st)
		s.byKey[st.Key] = st
	}
	return rows.Err()
}

func (s *Store) saveToSQL(st Study) error {
	if s.db == nil {
		return nil
	}
	visibility := string(st.Visibility)
	if visibility == "" {
		visibility = "private"
	}
	tier := string(st.Tier)
	if tier == "" {
		tier = "free"
	}
	_, err := s.db.Exec(
		"INSERT OR REPLACE INTO studies (key, owner, visibility, group_name, tier, title, emoji, where_clause, order_by, limit_num) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		st.Key, st.Owner, visibility, st.Group, tier, st.Title, st.Emoji, st.Where, st.OrderBy, st.Limit,
	)
	return err
}

// All returns a copy of every study.
func (s *Store) All() []Study {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Study(nil), s.all...)
}

// Get returns the study with the given key.
func (s *Store) Get(key string) (Study, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.byKey[key]
	return st, ok
}

// Upsert creates or replaces a study (by key), applying defaults and validating.
func (s *Store) Upsert(st Study) error {
	st.Key = strings.TrimSpace(st.Key)
	if st.Key == "" {
		return fmt.Errorf("key required")
	}
	if strings.TrimSpace(st.Where) == "" {
		return fmt.Errorf("where clause required")
	}
	if st.Owner == "" {
		st.Owner = user.GlobalID
	}
	if st.Tier == "" {
		st.Tier = user.TierFree
	}
	if st.Visibility == "" {
		if st.Owner == user.GlobalID {
			st.Visibility = VisPublic
		} else {
			st.Visibility = VisPrivate
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byKey[st.Key]; ok {
		for i := range s.all {
			if s.all[i].Key == st.Key {
				s.all[i] = st
				break
			}
		}
	} else {
		s.all = append(s.all, st)
	}
	s.byKey[st.Key] = st
	if err := s.saveToSQL(st); err != nil {
		return err
	}
	return nil
}

// Delete removes a study.
func (s *Store) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byKey[key]; !ok {
		return fmt.Errorf("study %q not found", key)
	}
	delete(s.byKey, key)
	if s.db != nil {
		s.db.Exec("DELETE FROM studies WHERE key = ?", key)
	}
	out := s.all[:0]
	for _, st := range s.all {
		if st.Key != key {
			out = append(out, st)
		}
	}
	s.all = out
	return nil
}

// ExportJSONL writes all studies as JSONL to w.
func (s *Store) ExportJSONL(w io.Writer) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	enc := json.NewEncoder(w)
	for _, st := range s.all {
		if err := enc.Encode(st); err != nil {
			return err
		}
	}
	return nil
}

// ImportJSONL imports studies from a JSONL file. Existing studies are updated.
func (s *Store) ImportJSONL(path string) error {
	return s.importJSONLFile(path)
}

// ExportJSON writes all studies as a wrapped JSON object with metadata.
func (s *Store) ExportJSON(w io.Writer) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return iohelp.ExportJSON(w, "studies", s.all, len(s.all))
}

// ImportJSON imports studies from a wrapped JSON object.
func (s *Store) ImportJSON(r io.Reader) (int, error) {
	exp, err := iohelp.ImportJSON(r)
	if err != nil {
		return 0, err
	}
	data, _ := json.Marshal(exp.Items)
	var studies []Study
	if err := json.Unmarshal(data, &studies); err != nil {
		return 0, fmt.Errorf("parse studies: %w", err)
	}
	imported := 0
	for _, st := range studies {
		if err := s.Upsert(st); err != nil {
			continue
		}
		imported++
	}
	return imported, nil
}
