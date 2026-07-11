package study

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"cetus-marketdata-scanner/internal/user"
)

// Store is a mutable set of studies backed by SQLite with JSONL fallback.
type Store struct {
	path  string
	db    *sql.DB // nil = JSONL-only (legacy)
	mu    sync.Mutex
	all   []Study
	byKey map[string]Study
}

// OpenStore loads studies from JSONL path (legacy, no DB).
func OpenStore(path string) (*Store, error) {
	return OpenStoreWithDB(path, nil)
}

// OpenStoreWithDB opens a SQLite-backed study store.
func OpenStoreWithDB(path string, db *sql.DB) (*Store, error) {
	s := &Store{path: path, db: db, byKey: map[string]Study{}}

	if db != nil {
		if err := s.loadFromSQL(); err != nil {
			return nil, fmt.Errorf("load studies from SQLite: %w", err)
		}
		if len(s.all) == 0 {
			if err := s.loadFromJSONL(); err != nil {
				return nil, err
			}
			for _, st := range s.all {
				s.saveToSQL(st)
			}
		}
		return s, nil
	}

	if err := s.loadFromJSONL(); err != nil {
		return nil, err
	}
	return s, nil
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

func (s *Store) loadFromJSONL() error {
	f, err := os.Open(s.path)
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
	s.all = all
	for _, st := range all {
		s.byKey[st.Key] = st
	}
	return nil
}

func (s *Store) saveToSQL(st Study) {
	if s.db == nil {
		return
	}
	visibility := string(st.Visibility)
	if visibility == "" {
		visibility = "private"
	}
	tier := string(st.Tier)
	if tier == "" {
		tier = "free"
	}
	s.db.Exec(
		"INSERT OR REPLACE INTO studies (key, owner, visibility, group_name, tier, title, emoji, where_clause, order_by, limit_num) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		st.Key, st.Owner, visibility, st.Group, tier, st.Title, st.Emoji, st.Where, st.OrderBy, st.Limit,
	)
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
	s.saveToSQL(st)
	return s.save()
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
	return s.save()
}

// save atomically rewrites the JSONL file. Caller holds the lock.
func (s *Store) save() error {
	tmp := s.path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	fmt.Fprintln(w, "# Studies — one JSON per line. SQL WHERE over the snapshot. Managed by the admin study editor.")
	enc := json.NewEncoder(w)
	for _, st := range s.all {
		if err := enc.Encode(st); err != nil {
			f.Close()
			return err
		}
	}
	if err := w.Flush(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
