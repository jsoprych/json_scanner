package study

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// Subscription represents a user's subscription to a study.
type Subscription struct {
	UserID   string `json:"user_id"`
	StudyKey string `json:"study_key"`
}

// SubscriptionStore manages user subscriptions to studies.
type SubscriptionStore struct {
	path string
	mu   sync.RWMutex
	subs map[string]map[string]bool // userID -> studyKey -> exists
}

// OpenSubscriptionStore opens or creates a subscription store at the given path.
func OpenSubscriptionStore(path string) (*SubscriptionStore, error) {
	s := &SubscriptionStore{
		path: path,
		subs: make(map[string]map[string]bool),
	}

	// Try to load existing subscriptions
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var sub Subscription
		if err := json.Unmarshal(scanner.Bytes(), &sub); err != nil {
			continue // skip malformed lines
		}
		if s.subs[sub.UserID] == nil {
			s.subs[sub.UserID] = make(map[string]bool)
		}
		s.subs[sub.UserID][sub.StudyKey] = true
	}

	return s, scanner.Err()
}

// Subscribe adds a subscription for a user to a study.
func (s *SubscriptionStore) Subscribe(userID, studyKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.subs[userID] == nil {
		s.subs[userID] = make(map[string]bool)
	}
	s.subs[userID][studyKey] = true

	return s.save()
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

	return s.save()
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

// save writes all subscriptions to disk.
func (s *SubscriptionStore) save() error {
	f, err := os.Create(s.path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	for userID, studies := range s.subs {
		for studyKey := range studies {
			if err := enc.Encode(Subscription{
				UserID:   userID,
				StudyKey: studyKey,
			}); err != nil {
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
	return s.save()
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
	return s.save()
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
