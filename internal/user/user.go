// Package user is a minimal identity that owns saved studies (and soon: watchlists,
// alerts, profiles). Real multi-user accounts land later; for now everything is
// owned by the Global user, so ownership is wired through the data model from day
// one and nothing has to be reshaped when accounts arrive.
package user

// Tier is a subscription level. Higher tiers can access more studies. A user can
// see a study when the study's tier rank ≤ the user's tier rank.
type Tier string

const (
	TierFree Tier = "free"
	TierPro  Tier = "pro"
)

// Rank orders tiers (higher = more access). Unknown/empty ranks as free.
func (t Tier) Rank() int {
	if t == TierPro {
		return 1
	}
	return 0
}

// User owns studies and other saved entities, at a subscription tier.
type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Tier Tier   `json:"tier"`
}

// GlobalID is the owner id for shared, system-owned entities.
const GlobalID = "global"

// Global is the single current user until real accounts exist — top tier, sees all.
func Global() User { return User{ID: GlobalID, Name: "Global", Tier: TierPro} }
