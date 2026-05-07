package writefreely

import "testing"

func TestIsValid(t *testing.T) {
	tests := []struct {
		v    string
		want bool
	}{
		{"v1.0.0", true},
		{"v0.0.1", true},
		{"v1.2.3", true},
		{"v1", true},
		{"v1.2", true},
		{"v1.0.0-alpha", true},
		{"v1.0.0-alpha.1", true},
		{"v1.0.0+build1", true},   // build metadata without dots
		{"v1.0.0+build.1", false}, // this impl doesn't allow dots in build metadata
		{"v1.0.0-alpha+001", true},
		{"", false},
		{"1.0.0", false},    // missing v prefix
		{"v1.0.0.0", false}, // extra patch segment
		{"v01.0.0", false},  // leading zero in major
		{"vx.y.z", false},   // non-numeric parts
	}

	for _, tt := range tests {
		t.Run(tt.v, func(t *testing.T) {
			got := IsValid(tt.v)
			if got != tt.want {
				t.Errorf("IsValid(%q) = %v, want %v", tt.v, got, tt.want)
			}
		})
	}
}

func TestCompareSemver(t *testing.T) {
	tests := []struct {
		name string
		v, w string
		want int
	}{
		{"equal versions", "v1.0.0", "v1.0.0", 0},
		{"major bump", "v2.0.0", "v1.0.0", 1},
		{"major behind", "v1.0.0", "v2.0.0", -1},
		{"minor bump", "v1.1.0", "v1.0.0", 1},
		{"minor behind", "v1.0.0", "v1.1.0", -1},
		{"patch bump", "v1.0.1", "v1.0.0", 1},
		{"patch behind", "v1.0.0", "v1.0.1", -1},
		{"release > prerelease", "v1.0.0", "v1.0.0-alpha", 1},
		{"prerelease < release", "v1.0.0-alpha", "v1.0.0", -1},
		{"both invalid", "bad", "alsobad", 0},
		{"first invalid", "bad", "v1.0.0", -1},
		{"second invalid", "v1.0.0", "bad", 1},
		{"short form v1 == v1.0.0", "v1", "v1.0.0", 0},
		{"short form v1.2 == v1.2.0", "v1.2", "v1.2.0", 0},
		{"v1.2.3 > v1.2", "v1.2.3", "v1.2", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CompareSemver(tt.v, tt.w)
			if got != tt.want {
				t.Errorf("CompareSemver(%q, %q) = %d, want %d", tt.v, tt.w, got, tt.want)
			}
		})
	}
}

// TestIsValid_EdgeCases targets uncovered branches in semParse, parsePrerelease, and parseBuild.
func TestIsValid_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		v    string
		want bool
	}{
		// semParse: bad minor prefix — non-dot character after parsed major
		{"bad minor prefix v1a", "v1a.0.0", false},
		// semParse: bad patch prefix — non-dot after parsed minor
		{"bad patch prefix v1.2a", "v1.2a.0", false},
		// semParse: bad minor version — dot followed by non-digit
		{"bad minor version v1.x.0", "v1.x.0", false},
		// semParse: bad patch version — dot followed by non-digit
		{"bad patch version v1.0.x", "v1.0.x", false},

		// parsePrerelease: leading zero in a numeric identifier is invalid
		{"prerelease with leading zero", "v1.0.0-alpha.01", false},
		// parsePrerelease: trailing dot (empty identifier after dot)
		{"prerelease trailing dot", "v1.0.0-alpha.", false},
		// parsePrerelease: double dot (empty identifier between dots)
		{"prerelease double dot", "v1.0.0-alpha..beta", false},
		// parsePrerelease: invalid character
		{"prerelease invalid char", "v1.0.0-alpha@1", false},
		// parsePrerelease: valid multi-segment prerelease
		{"prerelease numeric segment", "v1.0.0-1.2.3", true},

		// parseBuild: valid build metadata (no dots allowed in this impl)
		{"valid build metadata", "v1.0.0+build123", true},
		// parseBuild: build with invalid character
		{"build with space", "v1.0.0+build 1", false},

		// semParse: junk on end after valid prerelease+build
		{"junk after build", "v1.0.0+build1junk!", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValid(tt.v)
			if got != tt.want {
				t.Errorf("IsValid(%q) = %v, want %v", tt.v, got, tt.want)
			}
		})
	}
}
