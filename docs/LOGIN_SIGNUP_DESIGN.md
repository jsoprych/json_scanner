# Login/Signup Module — Design Document

## Current State

| Feature | Status |
|---------|--------|
| Login form (username + password) | ✅ |
| Session-based auth (cookie: `cetus_session`) | ✅ |
| Password hashing (PBKDF2-HMAC-SHA256, 600k iters) | ✅ |
| Account lockout (5 fails → 15 min lock, SQLite) | ✅ |
| Login rate limiting (5/min per IP, in-memory) | ✅ |
| Password visibility toggle (eye icon) | ✅ |
| Account disable (admin) | ✅ |
| Access logging (method, path, status, duration, size, IP) | ✅ |

## Required Additions

### 1. Signup Page (Self-Register)

**Flow:**
```
GET /signup → form (username, password, confirm password)
POST /signup → validate → create user → auto-login → redirect to /
```

**Validation rules:**
- Username: 3-32 chars, alphanumeric + underscore, unique
- Password: see §2
- Confirm password must match
- Rate limit: 3 signups per hour per IP

**Defaults for new users:**
- Role: `user` (free tier)
- Limits: `rates.Default()` (3 studies, 60 API/min, etc.)
- Visibility: private

**API endpoint:** `POST /api/v1/auth/signup`

**Config flag:** `SCANNER_OPEN_SIGNUP=true` (default false — admin-only creation unless explicitly opened)

### 2. Password Strength

**Requirements:**
- Minimum 8 characters (configurable: `SCANNER_MIN_PW_LENGTH`)
- At least one uppercase letter
- At least one digit or special character
- Not in common password blocklist (top 1000)

**UI feedback:**
- Real-time strength meter (red → yellow → green) as user types
- Text hints: "Too short", "Add an uppercase letter", "Strong!"

**Implementation:**
```go
// internal/auth/password.go
func ValidatePassword(pw string) error {
    if len(pw) < 8 { return errTooShort }
    if !hasUpper(pw) { return errNoUpper }
    if !hasDigit(pw) && !hasSpecial(pw) { return errNoDigitOrSpecial }
    if isCommon(pw) { return errCommonPassword }
    return nil
}
```

### 3. Change Password (Self-Service)

**Flow:**
```
GET /profile → current user info + change password form
POST /profile/password → validate current password → set new password
```

**Requirements:**
- Must provide current password
- New password must pass strength check (§2)
- Confirm new password
- Log the change (audit trail)
- Invalidate all existing sessions for security

**UI:** Simple form on user profile/dashboard page.

**API endpoint:** `PUT /api/v1/auth/password`

### 4. Password Reset (Admin-Assisted)

Since we have no email infrastructure, reset is admin-initiated.

**Flow:**
```
User requests reset (via admin or support) →
Admin navigates to Users → clicks "Reset Password" →
System generates temporary password →
Admin shares with user (out of band) →
User logs in with temp password →
System forces password change on next login
```

**Implementation:**
- Add `force_password_change` boolean to `users` table
- On admin reset: set `force_password_change=true`, set temporary password
- On login: if `force_password_change`, redirect to `/profile/password`
- New password must differ from temporary

**Admin UI:** Button on Users page → "Reset Password" → shows temp password once → user changes on next login

**API endpoint:** `POST /api/v1/users/{id}/reset-password` (admin only)

## Social Login (Google, Facebook, etc.)

### Feasibility

**Technically easy.** Go's `golang.org/x/oauth2` handles the OAuth2 flow:
1. User clicks "Sign in with Google"
2. Redirect to Google's consent screen
3. Google redirects back with auth code
4. Server exchanges code for access token
5. Server fetches user profile (email, name)
6. Create or match local user account
7. Log in with session cookie

**Code required:** ~150 lines per provider. Google is the best starting point.

### Prerequisites

| Requirement | Current | What's Needed |
|-------------|---------|---------------|
| Public domain | ❌ (think:8080) | Domain + DNS (e.g., chartgeometry.com) |
| HTTPS | ❌ (plain HTTP) | TLS cert (Let's Encrypt or Cloudflare) |
| OAuth credentials | ❌ | Google Cloud Console → API keys |
| OAuth library | ❌ | `golang.org/x/oauth2` |

### Implementation Plan

**Phase 1 (now):** Basic signup/password (no social login)
**Phase 2 (when public):** Google OAuth2 (most common, easiest to set up)
**Phase 3 (later):** Facebook, GitHub — same OAuth2 pattern

### Google OAuth2 Flow (Phase 2)

```go
// internal/auth/oauth.go
var googleOAuth = &oauth2.Config{
    ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
    ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
    RedirectURL:  "https://chartgeometry.com/auth/google/callback",
    Scopes:       []string{"email", "profile"},
    Endpoint:     google.Endpoint,
}

// GET /auth/google → redirect to Google
// GET /auth/google/callback → exchange code → create/find user → login
```

### Migration: Adding Social Login Later

Existing users are unaffected. New users can choose:
- Sign up with email/password (current flow)
- Sign up with Google (adds `oauth_provider` + `oauth_id` fields to users table)

Schema addition:
```sql
ALTER TABLE users ADD COLUMN oauth_provider TEXT DEFAULT '';
ALTER TABLE users ADD COLUMN oauth_id TEXT DEFAULT '';
CREATE UNIQUE INDEX idx_users_oauth ON users(oauth_provider, oauth_id) WHERE oauth_provider != '';
```

## Schema Changes

### New columns on `users`:
```sql
ALTER TABLE users ADD COLUMN force_password_change INTEGER NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN temp_password_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN oauth_provider TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN oauth_id TEXT NOT NULL DEFAULT '';
```

### New table: `password_audit` (optional, for compliance):
```sql
CREATE TABLE password_audit (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    action TEXT NOT NULL,   -- 'change', 'reset', 'set'
    changed_by TEXT,         -- admin who performed reset
    changed_at INTEGER NOT NULL
);
```

## UI Mockups

### Signup Page
```
┌──────────────────────────────────────┐
│           📡 Cetus Scanner            │
│                                       │
│  ┌─ Create Account ─────────────────┐ │
│  │ User:  [_______________]         │ │
│  │ Password: [___________] 👁        │ │
│  │ Confirm:  [___________]          │ │
│  │                                  │ │
│  │ Strength: ████████░░ Strong      │ │
│  │                                  │ │
│  │ [ Create Account ]               │ │
│  │                                  │ │
│  │ Already have an account? Sign in  │ │
│  └──────────────────────────────────┘ │
└──────────────────────────────────────┘
```

### Change Password (Profile Page)
```
┌─ Change Password ────────────────────┐
│ Current Password: [___________] 👁    │
│ New Password:     [___________] 👁    │
│ Confirm:          [___________]      │
│                                       │
│ Strength: ██████████░░ Very Strong   │
│                                       │
│ [ Change Password ]                   │
└───────────────────────────────────────┘
```

## Implementation Priority

| Priority | Feature | Effort | Depends On |
|----------|---------|--------|------------|
| **P0** | Password strength validation | 1h | None |
| **P0** | Change password (self) | 2h | P0 |
| **P1** | Signup page | 3h | P0 |
| **P1** | Password reset (admin) | 2h | None |
| **P2** | Social login (Google) | 3h | Domain + HTTPS |

## Config Flags (`.env`)

```bash
SCANNER_OPEN_SIGNUP=false       # allow self-registration
SCANNER_MIN_PW_LENGTH=8         # minimum password length
SCANNER_SIGNUP_RATE_LIMIT=3     # signups per hour per IP
```

## Execution Plan

**Session 1 (4h):**
1. Password strength validation (`internal/auth/password.go`)
2. Change password endpoint + UI
3. Schema migration v5 (new user columns)

**Session 2 (5h):**
4. Signup page + endpoint
5. Rate limiting for signups
6. Password reset (admin-initiated)
7. Force password change on next login

**Session 3 (future):**
8. Social login (Google OAuth2)
9. OAuth migration

---

**Ready to build Phase 1?** Start with password strength + change password — the foundation for everything else.
