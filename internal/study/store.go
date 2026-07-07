package study

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"cetus-marketdata-scanner/internal/user"
)

// Store is a mutable, JSONL-file-backed set of studies, safe for concurrent use.
// The admin study editor creates/updates/deletes through it; every mutation
// atomically rewrites the file, so edits take effect without a restart.
type Store struct {
	path  string
	mu    sync.Mutex
	all   []Study
	byKey map[string]Study
}

// OpenStore loads studies from path (a missing file yields an empty store).
func OpenStore(path string) (*Store, error) {
	s := &Store{path: path, byKey: map[string]Study{}}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	defer f.Close()
	all, err := LoadJSONL(f)
	if err != nil {
		return nil, err
	}
	s.all = all
	for _, st := range all {
		s.byKey[st.Key] = st
	}
	return s, nil
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
