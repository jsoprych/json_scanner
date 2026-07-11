package permissions

import (
	"testing"
)

func TestHasPermission(t *testing.T) {
	tests := []struct {
		perms, action int
		want           bool
	}{
		{7, 4, true},  // rwx + read
		{7, 2, true},  // rwx + write
		{7, 1, true},  // rwx + delete
		{5, 4, true},  // r-x + read
		{5, 2, false}, // r-x + write (no w)
		{5, 1, true},  // r-x + delete
		{4, 4, true},  // r-- + read
		{4, 2, false}, // r-- + write
		{0, 4, false}, // --- + read
	}
	for _, tc := range tests {
		got := HasPermission(tc.perms, tc.action)
		if got != tc.want {
			t.Errorf("HasPermission(%d, %d) = %v, want %v", tc.perms, tc.action, got, tc.want)
		}
	}
}

func TestAddRemovePermission(t *testing.T) {
	p := 4 // read only
	p = AddPermission(p, PermWrite)
	if p != 6 {
		t.Errorf("add write: got %d want 6", p)
	}
	p = RemovePermission(p, PermRead)
	if p != 2 {
		t.Errorf("remove read: got %d want 2", p)
	}
	p = RemovePermission(p, PermWrite)
	if p != 0 {
		t.Errorf("remove write: got %d want 0", p)
	}
}

func TestPermissionString(t *testing.T) {
	tests := []struct {
		perms int
		want  string
	}{
		{7, "rwx"},
		{6, "rw-"},
		{5, "r-x"},
		{4, "r--"},
		{3, "-wx"},
		{2, "-w-"},
		{1, "--x"},
		{0, "---"},
	}
	for _, tc := range tests {
		got := PermissionString(tc.perms)
		if got != tc.want {
			t.Errorf("PermissionString(%d) = %q, want %q", tc.perms, tc.want, got)
		}
	}
}

func TestDefaultPermissions(t *testing.T) {
	owner, group, all := DefaultPermissions()
	if owner != PermFull || group != PermNone || all != PermNone {
		t.Errorf("defaults: %d/%d/%d want 7/0/0", owner, group, all)
	}
}

func TestValidPermission(t *testing.T) {
	for i := 0; i <= 7; i++ {
		if !ValidPermission(i) {
			t.Errorf("ValidPermission(%d) = false", i)
		}
	}
	if ValidPermission(-1) || ValidPermission(8) {
		t.Error("ValidPermission out of range returned true")
	}
}

func TestParsePermissionString(t *testing.T) {
	tests := []struct {
		s    string
		want int
	}{
		{"rwx", 7},
		{"rw-", 6},
		{"r-x", 5},
		{"r--", 4},
		{"-wx", 3},
		{"-w-", 2},
		{"--x", 1},
		{"---", 0},
	}
	for _, tc := range tests {
		got, _ := ParsePermissionString(tc.s)
		if got != tc.want {
			t.Errorf("ParsePermissionString(%q) = %d, want %d", tc.s, got, tc.want)
		}
	}
}
