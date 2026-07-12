package serve

import "cetus-marketdata-scanner/internal/auth"

// loginGuard wraps auth.LoginGuard for the serve package.
type loginGuard = auth.LoginGuard
