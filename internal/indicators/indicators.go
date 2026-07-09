// Package indicators holds the pure technical-indicator functions the scanner
// materializes. Every function is PURE and NO-LOOKAHEAD: given a series indexed
// oldest→newest, the value at index i is computed from bars < i (excluding the
// current bar). This ensures indicators represent only information available
// BEFORE the current bar's close. Warm-up positions (before the window is full)
// are math.NaN, so a half-warmed symbol simply fails any comparison rather than
// matching on a partial value.
//
// This is the Phase-1 subset (see docs/PHASE1_MVP.md) — the seven features the
// daily digest needs. The full catalog (docs/INDICATORS.md) grows from here.
package indicators
