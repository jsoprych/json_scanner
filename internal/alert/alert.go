package alert

import (
	"cetus-marketdata-scanner/internal/snapshot"
	"cetus-marketdata-scanner/internal/study"
)

// Match represents a symbol that matched a study on a specific date.
type Match struct {
	Symbol string `json:"symbol"`
	Date   int64  `json:"date"`
}

// Entry represents a symbol that entered a study (matched today, not yesterday).
type Entry struct {
	Symbol    string `json:"symbol"`
	StudyKey  string `json:"study_key"`
	Date      int64  `json:"date"`
	PrevDate  int64  `json:"prev_date,omitempty"`
}

// Exit represents a symbol that exited a study (matched yesterday, not today).
type Exit struct {
	Symbol    string `json:"symbol"`
	StudyKey  string `json:"study_key"`
	Date      int64  `json:"date"`
	PrevDate  int64  `json:"prev_date"`
}

// Detector detects entries and exits for studies across snapshots.
type Detector struct {
	snap *snapshot.DB
}

// NewDetector creates a new alert detector.
func NewDetector(snap *snapshot.DB) *Detector {
	return &Detector{snap: snap}
}

// DetectEntries finds symbols that entered a study on the given date.
// A symbol enters if it matches on date but not on prevDate.
func (d *Detector) DetectEntries(s study.Study, date, prevDate int64) ([]Entry, error) {
	// Get matches for current date
	if err := d.snap.SetActive(date); err != nil {
		return nil, err
	}
	currentMatches, err := d.snap.Run(s)
	if err != nil {
		return nil, err
	}

	// Get matches for previous date
	if err := d.snap.SetActive(prevDate); err != nil {
		// If previous date doesn't exist, all current matches are entries
		entries := make([]Entry, 0, len(currentMatches))
		for _, m := range currentMatches {
			entries = append(entries, Entry{
				Symbol:   m.Symbol,
				StudyKey: s.Key,
				Date:     date,
			})
		}
		return entries, nil
	}
	prevMatches, err := d.snap.Run(s)
	if err != nil {
		return nil, err
	}

	// Build set of previous matches
	prevSet := make(map[string]bool, len(prevMatches))
	for _, m := range prevMatches {
		prevSet[m.Symbol] = true
	}

	// Find entries: in current but not in previous
	entries := make([]Entry, 0)
	for _, m := range currentMatches {
		if !prevSet[m.Symbol] {
			entries = append(entries, Entry{
				Symbol:   m.Symbol,
				StudyKey: s.Key,
				Date:     date,
				PrevDate: prevDate,
			})
		}
	}

	return entries, nil
}

// DetectExits finds symbols that exited a study on the given date.
// A symbol exits if it matched on prevDate but not on date.
func (d *Detector) DetectExits(s study.Study, date, prevDate int64) ([]Exit, error) {
	// Get matches for current date
	if err := d.snap.SetActive(date); err != nil {
		return nil, err
	}
	currentMatches, err := d.snap.Run(s)
	if err != nil {
		return nil, err
	}

	// Get matches for previous date
	if err := d.snap.SetActive(prevDate); err != nil {
		// If previous date doesn't exist, no exits
		return nil, nil
	}
	prevMatches, err := d.snap.Run(s)
	if err != nil {
		return nil, err
	}

	// Build set of current matches
	currentSet := make(map[string]bool, len(currentMatches))
	for _, m := range currentMatches {
		currentSet[m.Symbol] = true
	}

	// Find exits: in previous but not in current
	exits := make([]Exit, 0)
	for _, m := range prevMatches {
		if !currentSet[m.Symbol] {
			exits = append(exits, Exit{
				Symbol:   m.Symbol,
				StudyKey: s.Key,
				Date:     date,
				PrevDate: prevDate,
			})
		}
	}

	return exits, nil
}

// DetectChanges finds both entries and exits for a study between two dates.
func (d *Detector) DetectChanges(s study.Study, date, prevDate int64) ([]Entry, []Exit, error) {
	entries, err := d.DetectEntries(s, date, prevDate)
	if err != nil {
		return nil, nil, err
	}
	exits, err := d.DetectExits(s, date, prevDate)
	if err != nil {
		return nil, nil, err
	}
	return entries, exits, nil
}
