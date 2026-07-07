package study

import (
	"strings"
	"testing"

	"cetus-marketdata-scanner/internal/user"
)

func TestLoadDefaultsAndAccess(t *testing.T) {
	data := `
# a comment line
{"key":"a","title":"A","where":"close>sma200","limit":5}
{"owner":"global","tier":"pro","key":"b","title":"B","where":"rsi14<30"}
{"owner":"alice","key":"c","title":"C","where":"1=1"}
`
	studies, err := LoadJSONL(strings.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(studies) != 3 {
		t.Fatalf("got %d studies", len(studies))
	}
	// defaults: missing owner → global, missing tier → free.
	if studies[0].Owner != user.GlobalID || studies[0].Tier != user.TierFree {
		t.Errorf("defaults not applied: %+v", studies[0])
	}

	// free user: only global free study (a).
	free := Accessible(studies, user.User{ID: "bob", Tier: user.TierFree})
	if len(free) != 1 || free[0].Key != "a" {
		t.Errorf("free access = %+v", free)
	}
	// pro user: global free + global pro (a, b), not alice's private (c).
	pro := Accessible(studies, user.User{ID: "bob", Tier: user.TierPro})
	if len(pro) != 2 {
		t.Errorf("pro access = %d, want 2", len(pro))
	}
	// owner sees their own study even though it's private.
	alice := Accessible(studies, user.User{ID: "alice", Tier: user.TierFree})
	if len(alice) != 2 { // global free (a) + own (c)
		t.Errorf("alice access = %d, want 2", len(alice))
	}
	// admin sees everything, regardless of owner or tier (even at free tier).
	admin := Accessible(studies, user.User{ID: "root", Tier: user.TierFree, Role: user.RoleAdmin})
	if len(admin) != 3 {
		t.Errorf("admin access = %d, want 3 (all)", len(admin))
	}
}

func TestGroupVisibility(t *testing.T) {
	studies := []Study{
		{Owner: user.GlobalID, Visibility: VisPublic, Tier: user.TierFree, Key: "pub"},
		{Owner: user.GlobalID, Visibility: VisGroup, Group: "desk-a", Tier: user.TierFree, Key: "grp"},
	}
	// not in the group → only the public study.
	out := Accessible(studies, user.User{ID: "bob", Tier: user.TierFree})
	if len(out) != 1 || out[0].Key != "pub" {
		t.Errorf("non-member access = %+v", out)
	}
	// group member → both.
	out = Accessible(studies, user.User{ID: "alice", Tier: user.TierFree, Groups: []string{"desk-a"}})
	if len(out) != 2 {
		t.Errorf("member access = %d, want 2", len(out))
	}
}
