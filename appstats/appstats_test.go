package appstats

import (
	"strings"
	"testing"
	"time"
)

func TestTimeSincePro(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name     string
		then     time.Time
		contains []string // all substrings expected in result
	}{
		{
			name:     "future time returns future",
			then:     now.Add(1 * time.Hour),
			contains: []string{"future"},
		},
		{
			name:     "zero diff returns empty string",
			then:     now,
			contains: []string{},
		},
		{
			name:     "1 second ago",
			then:     now.Add(-1 * time.Second),
			contains: []string{"1 second"},
		},
		{
			name:     "30 seconds ago",
			then:     now.Add(-30 * time.Second),
			contains: []string{"seconds"},
		},
		{
			name:     "1 minute ago",
			then:     now.Add(-1 * time.Minute),
			contains: []string{"1 minute"},
		},
		{
			name:     "45 minutes ago",
			then:     now.Add(-45 * time.Minute),
			contains: []string{"minutes"},
		},
		{
			name:     "1 hour ago",
			then:     now.Add(-1 * time.Hour),
			contains: []string{"1 hour"},
		},
		{
			name:     "5 hours ago",
			then:     now.Add(-5 * time.Hour),
			contains: []string{"hours"},
		},
		{
			name:     "1 day ago",
			then:     now.Add(-24 * time.Hour),
			contains: []string{"1 day"},
		},
		{
			name:     "3 days ago",
			then:     now.Add(-72 * time.Hour),
			contains: []string{"days"},
		},
		{
			name:     "1 week ago",
			then:     now.Add(-7 * 24 * time.Hour),
			contains: []string{"1 week"},
		},
		{
			name:     "2 weeks ago",
			then:     now.Add(-14 * 24 * time.Hour),
			contains: []string{"weeks"},
		},
		{
			name:     "1 month ago",
			then:     now.Add(-30 * 24 * time.Hour),
			contains: []string{"1 month"},
		},
		{
			name:     "6 months ago",
			then:     now.Add(-180 * 24 * time.Hour),
			contains: []string{"months"},
		},
		{
			name:     "1 year ago",
			then:     now.Add(-365 * 24 * time.Hour),
			contains: []string{"1 year"},
		},
		{
			name:     "3 years ago",
			then:     now.Add(-3 * 365 * 24 * time.Hour),
			contains: []string{"years"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TimeSincePro(tt.then)
			for _, want := range tt.contains {
				if !strings.Contains(got, want) {
					t.Errorf("TimeSincePro() = %q, want it to contain %q", got, want)
				}
			}
		})
	}
}

func TestFileSize(t *testing.T) {
	tests := []struct {
		name     string
		size     int64
		expected string
	}{
		{"bytes", 5, "5 B"},
		{"kilobytes", 1024, "1.0 KB"},
		{"megabytes", 1024 * 1024, "1.0 MB"},
		{"gigabytes", 1024 * 1024 * 1024, "1.0 GB"},
		{"large megabytes", 50 * 1024 * 1024, "50 MB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FileSize(tt.size)
			if got != tt.expected {
				t.Errorf("FileSize(%d) = %q, want %q", tt.size, got, tt.expected)
			}
		})
	}
}
