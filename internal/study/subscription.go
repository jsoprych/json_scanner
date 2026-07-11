package study

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
)

// Subscription represents a user's subscription to a study.
type Subscription struct {
	UserID   string `json:"user_id"`
	StudyKey string `json:"study_key"`
}

// SubscriptionStore manages user subscriptions backed by SQLite.
// JSONL files are for import/export only — NOT synced on writes.
type SubscriptionStore struct {
	db   *sql.DB
	mu   sync.RWMutex
	subs map[string]map[string]bool
}

// OpenSubscriptionStore opens a subscription store from JSONL path (legacy, no DB).
func OpenSubscriptionStore(path string) (*SubscriptionStore, error) {
	return OpenSubscriptionStoreWithDB(nil, path)
}

// OpenSubscriptionStoreWithDB opens a SQLite-backed subscription store.
func OpenSubscriptionStoreWithDB(db *sql.DB, jsonlPath string) (*SubscriptionStore, error) {
	s := &SubscriptionStore{
		db:   db,
		subs: make(map[string]map[string]bool),
	}

	if db != nil {
		if err := s.loadFromSQL(); err != nil {
			return nil, fmt.Errorf("load subscriptions from SQLite: %w", err)
		}
		if len(s.subs) == 0 && jsonlPath != "" {
			if err := s.importJSONLFile(jsonlPath); err != nil {
				return nil, err
			}
			s.seedSQL()
		}
		return s, nil
	}

	if jsonlPath != "" {
		if err := s.importJSONLFile(jsonlPath); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *SubscriptionStore) importJSONLFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := splitLines(string(data))
	for _, line := range lines {
		var sub Subscription
		if err := json.Unmarshal([]byte(line), &sub); err != nil {
			continue
		}
		if s.subs[sub.UserID] == nil {
			s.subs[sub.UserID] = make(map[string]bool)
		}
		s.subs[sub.UserID][sub.StudyKey] = true
	}
	return nil
}

func (s *SubscriptionStore) loadFromSQL() error {
	rows, err := s.db.Query("SELECT user_id, study_key FROM subscriptions ORDER BY user_id, study_key")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var userID, studyKey string
		if err := rows.Scan(&userID, &studyKey); err != nil {
			return err
		}
		if s.subs[userID] == nil {
			s.subs[userID] = make(map[string]bool)
		}
		s.subs[userID][studyKey] = true
	}
	return rows.Err()
}

func (s *SubscriptionStore) seedSQL() {
	if s.db == nil {
		return
	}
	for userID, studies := range s.subs {
		for studyKey := range studies {
			s.db.Exec("INSERT OR IGNORE INTO subscriptions (user_id, study_key) VALUES (?, ?)", userID, studyKey)
		}
	}
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line := trimSpace(s[start:i])
			if line != "" {
				lines = append(lines, line)
			}
			start = i + 1
		}
	}
	if start < len(s) {
		line := trimSpace(s[start:])
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\r' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\r' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}

// Subscribe adds a subscription for a user to a study.
func (s *SubscriptionStore) Subscribe(userID, studyKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.subs[userID] == nil {
		s.subs[userID] = make(map[string]bool)
	}
	s.subs[userID][studyKey] = true

	if s.db != nil {
		s.db.Exec("INSERT OR IGNORE INTO subscriptions (user_id, study_key) VALUES (?, ?)", userID, studyKey)
	}
	return nil
}

// Unsubscribe removes a subscription for a user from a study.
func (s *SubscriptionStore) Unsubscribe(userID, studyKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.subs[userID] != nil {
		delete(s.subs[userID], studyKey)
		if len(s.subs[userID]) == 0 {
			delete(s.subs, userID)
		}
	}

	if s.db != nil {
		s.db.Exec("DELETE FROM subscriptions WHERE user_id = ? AND study_key = ?", userID, studyKey)
	}
	return nil
}

// IsSubscribed checks if a user is subscribed to a study.
func (s *SubscriptionStore) IsSubscribed(userID, studyKey string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.subs[userID] == nil {
		return false
	}
	return s.subs[userID][studyKey]
}

// GetUserSubscriptions returns all study keys a user is subscribed to.
func (s *SubscriptionStore) GetUserSubscriptions(userID string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.subs[userID] == nil {
		return nil
	}

	keys := make([]string, 0, len(s.subs[userID]))
	for key := range s.subs[userID] {
		keys = append(keys, key)
	}
	return keys
}

// GetStudySubscribers returns all user IDs subscribed to a study.
func (s *SubscriptionStore) GetStudySubscribers(studyKey string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var users []string
	for userID, studies := range s.subs {
		if studies[studyKey] {
			users = append(users, userID)
		}
	}
	return users
}

// ExportJSONL writes all subscriptions as JSONL to w.
func (s *SubscriptionStore) ExportJSONL(w io.Writer) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	enc := json.NewEncoder(w)
	for userID, studies := range s.subs {
		for studyKey := range studies {
			if err := enc.Encode(Subscription{UserID: userID, StudyKey: studyKey}); err != nil {
				return err
			}
		}
	}
	return nil
}

// DeleteUserSubscriptions removes all subscriptions for a user.
func (s *SubscriptionStore) DeleteUserSubscriptions(userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.subs, userID)
	if s.db != nil {
		s.db.Exec("DELETE FROM subscriptions WHERE user_id = ?", userID)
	}
	return nil
}

// DeleteStudySubscriptions removes all subscriptions for a study.
func (s *SubscriptionStore) DeleteStudySubscriptions(studyKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for userID, studies := range s.subs {
		delete(studies, studyKey)
		if len(studies) == 0 {
			delete(s.subs, userID)
		}
	}
	if s.db != nil {
		s.db.Exec("DELETE FROM subscriptions WHERE study_key = ?", studyKey)
	}
	return nil
}

// Count returns the total number of subscriptions.
func (s *SubscriptionStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, studies := range s.subs {
		count += len(studies)
	}
	return count
}

// String returns a human-readable representation of the store.
func (s *SubscriptionStore) String() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return fmt.Sprintf("SubscriptionStore{users: %d, subscriptions: %d}", len(s.subs), s.Count())
}
