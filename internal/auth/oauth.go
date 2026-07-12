package auth

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
)

// GoogleEndpoint avoids cloud.google.com/go dependency.
var GoogleEndpoint = oauth2.Endpoint{
	AuthURL:  "https://accounts.google.com/o/oauth2/auth",
	TokenURL: "https://oauth2.googleapis.com/token",
}

// OAuthConfig holds configuration for a social login provider.
type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
	Endpoint     oauth2.Endpoint
}

// GoogleOAuth returns a pre-configured Google OAuth2 config.
func GoogleOAuth(clientID, clientSecret, redirectURL string) *OAuthConfig {
	return &OAuthConfig{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     GoogleEndpoint,
	}
}

var oauthStates = map[string]bool{}

func generateState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// OAuthHandler manages OAuth2 login flows.
type OAuthHandler struct {
	config *oauth2.Config
	// OnLogin is called when a user authenticates via OAuth.
	OnLogin func(email, name string) (userID string, err error)
}

// NewOAuthHandler creates an OAuth2 handler.
func NewOAuthHandler(cfg *OAuthConfig, onLogin func(email, name string) (string, error)) *OAuthHandler {
	return &OAuthHandler{
		config: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Scopes:       cfg.Scopes,
			Endpoint:     cfg.Endpoint,
		},
		OnLogin: onLogin,
	}
}

// LoginURL returns the OAuth provider's login URL.
func (h *OAuthHandler) LoginURL() string {
	state := generateState()
	oauthStates[state] = true
	return h.config.AuthCodeURL(state, oauth2.AccessTypeOnline)
}

// HandleCallback processes the OAuth2 callback.
func (h *OAuthHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	if !oauthStates[state] {
		http.Error(w, "invalid oauth state", http.StatusBadRequest)
		return
	}
	delete(oauthStates, state)

	code := r.URL.Query().Get("code")
	token, err := h.config.Exchange(context.Background(), code)
	if err != nil {
		http.Error(w, "oauth exchange failed: "+err.Error(), http.StatusBadRequest)
		return
	}

	client := h.config.Client(context.Background(), token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		http.Error(w, "failed to get user info", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var info struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		http.Error(w, "failed to parse user info", http.StatusInternalServerError)
		return
	}

	userID, err := h.OnLogin(info.Email, info.Name)
	if err != nil {
		http.Error(w, "login failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("X-OAuth-UserID", userID)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
