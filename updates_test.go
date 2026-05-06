package writefreely

import (
	"regexp"
	"testing"
	"time"
)

func TestUpdatesRoundTrip(t *testing.T) {
	cache := newUpdatesCache(defaultUpdatesCacheTime)
	t.Run("New Updates Cache", func(t *testing.T) {

		if cache == nil {
			t.Fatal("Returned nil cache")
		}

		if cache.frequency != defaultUpdatesCacheTime {
			t.Fatalf("Got cache expiry frequency: %s but expected: %s", cache.frequency, defaultUpdatesCacheTime)
		}

		if cache.currentVersion != "v"+softwareVer {
			t.Fatalf("Got current version: %s but expected: %s", cache.currentVersion, "v"+softwareVer)
		}
	})

	t.Run("Release URL", func(t *testing.T) {
		url := cache.ReleaseNotesURL()

		reg, err := regexp.Compile(`^https:\/\/blog.writefreely.org\/version(-\d+){1,}$`)
		if err != nil {
			t.Fatalf("Test Case Error: Failed to compile regex: %v", err)
		}
		match := reg.MatchString(url)

		if !match {
			t.Fatalf("Malformed Release URL: %s", url)
		}
	})

	t.Run("Check Now", func(t *testing.T) {
		// ensure time between init and next check
		time.Sleep(1 * time.Second)

		cache.mu.Lock()
		prevLastCheck := cache.lastCheck
		// force to known older version for latest and current
		prevLatestVer := "v0.8.1"
		cache.latestVersion = prevLatestVer
		cache.currentVersion = "v0.8.0"
		cache.mu.Unlock()

		err := cache.CheckNow()
		if err != nil {
			t.Fatalf("Error should be nil, got: %v", err)
		}

		cache.mu.Lock()
		lastCheck := cache.lastCheck
		latestVersion := cache.latestVersion
		cache.mu.Unlock()

		if prevLastCheck == lastCheck {
			t.Fatal("Expected lastCheck to update")
		}

		if lastCheck.Before(prevLastCheck) {
			t.Fatal("Last check should be newer than previous")
		}

		if prevLatestVer == latestVersion {
			t.Fatal("expected latestVersion to update")
		}

	})

	t.Run("Are Available", func(t *testing.T) {
		if !cache.AreAvailable() {
			cache.mu.Lock()
			current, latest := cache.currentVersion, cache.latestVersion
			cache.mu.Unlock()
			t.Fatalf("Cache reports no updates but Current is %s and Latest is %s", current, latest)
		}
	})

	t.Run("Latest Version", func(t *testing.T) {
		gotLatest := cache.LatestVersion()
		cache.mu.Lock()
		expected := cache.latestVersion
		cache.mu.Unlock()
		if gotLatest != expected {
			t.Fatalf("Malformed latest version. Expected: %s but got: %s", expected, gotLatest)
		}
	})
}
