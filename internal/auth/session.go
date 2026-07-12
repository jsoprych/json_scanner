package auth

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"
)

// SessionStore is an in-memory session store. Thread-safe.
type SessionStore struct {
	mu       sync.Mutex
	entries  map[string]SessionEntry
	cookieName string
}

// SessionEntry holds a session.
type SessionEntry struct {
	UserID string
	Expiry time.Time
}

// NewSessionStore creates a session store with the given cookie name.
func NewSessionStore(cookieName string) *SessionStore {
	return &SessionStore{
		entries:    make(map[string]SessionEntry),
		cookieName: cookieName,
	}
}

// Create creates a new session for userID with the given TTL.
// Returns the opaque session token.
func (s *SessionStore) Create(userID string, ttl time.Duration) string {
	b := make([]byte, 32)
	rand.Read(b)
	tok := hex.EncodeToString(b)
	s.mu.Lock()
	s.entries[tok] = SessionEntry{UserID: userID, Expiry: time.Now().Add(ttl)}
	s.mu.Unlock()
	return tok
}

// Get returns the userID if the session is valid.
func (s *SessionStore) Get(tok string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[tok]
	if !ok || time.Now().After(e.Expiry) {
		if ok {
			delete(s.entries, tok)
		}
		return "", false
	}
	return e.UserID, true
}

// Delete removes a session.
func (s *SessionStore) Delete(tok string) {
	s.mu.Lock()
	delete(s.entries, tok)
	s.mu.Unlock()
}

// SetCookie writes the session cookie to the response.
func (s *SessionStore) SetCookie(w http.ResponseWriter, tok string, ttl time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.cookieName,
		Value:    tok,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(ttl.Seconds()),
	})
}

// ClearCookie removes the session cookie.
func (s *SessionStore) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:   s.cookieName,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
}

// GetCookie returns the session token from the request cookie.
func (s *SessionStore) GetCookie(r *http.Request) (string, bool) {
	c, err := r.Cookie(s.cookieName)
	if err != nil {
		return "", false
	}
	return c.Value, true
}
