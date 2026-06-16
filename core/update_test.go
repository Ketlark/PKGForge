package core

import "testing"

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"v1.2.3", "1.2.3"},
		{"1.2.3", "1.2.3"},
		{" v1.0.0 ", "1.0.0"},
	}
	for _, tc := range tests {
		if got := normalizeVersion(tc.in); got != tc.want {
			t.Errorf("normalizeVersion(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestIsNewerVersion(t *testing.T) {
	tests := []struct {
		current, candidate string
		want               bool
	}{
		{"1.0.0", "1.0.1", true},
		{"1.0.0", "1.1.0", true},
		{"1.0.0", "2.0.0", true},
		{"1.2.3", "1.2.3", false},
		{"1.2.3", "1.2.2", false},
		{"1.0.0", "1.0.1", true},
		{"1.2.9", "1.2.10", true},
		{"v1.0.0", "1.0.1", true},
	}
	for _, tc := range tests {
		if got := isNewerVersion(tc.current, tc.candidate); got != tc.want {
			t.Errorf("isNewerVersion(%q, %q) = %v, want %v", tc.current, tc.candidate, got, tc.want)
		}
	}
}

func TestFindHashInChecksums(t *testing.T) {
	hash := "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234"
	content := hash + "  pkg-forge-linux-amd64\n"
	if got := findHashInChecksums(content, "pkg-forge-linux-amd64"); got != hash {
		t.Fatalf("findHashInChecksums = %q, want %q", got, hash)
	}
}
