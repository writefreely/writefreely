package writefreely

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"
)

func TestUpdatesRoundTrip(t *testing.T) {
	version, status := "v0.17.3", http.StatusOK
	withVersionServer(t, &version, &status)

	cache := newUpdatesCache(defaultUpdatesCacheTime)
	// wait for the initial background check to finish
	for cache.LastCheck().IsZero() {
		time.Sleep(10 * time.Millisecond)
	}
	t.Run("New Updates Cache", func(t *testing.T) {

		if cache == nil {
			t.Fatal("Returned nil cache")
		}

		if cache.frequency != defaultUpdatesCacheTime {
			t.Fatalf("Got cache expiry frequency: %s but expected: %s", cache.frequency, defaultUpdatesCacheTime)
		}

		if cache.currentVersion != releaseVersion("v"+softwareVer) {
			t.Fatalf("Got current version: %s but expected: %s", cache.currentVersion, releaseVersion("v"+softwareVer))
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

		prevLastCheck := cache.lastCheck

		// force to known older version for latest and current
		prevLatestVer := "v0.8.1"
		cache.latestVersion = prevLatestVer
		cache.currentVersion = "v0.8.0"

		err := cache.CheckNow()
		if err != nil {
			t.Fatalf("Error should be nil, got: %v", err)
		}

		if prevLastCheck == cache.lastCheck {
			t.Fatal("Expected lastCheck to update")
		}

		if cache.lastCheck.Before(prevLastCheck) {
			t.Fatal("Last check should be newer than previous")
		}

		if prevLatestVer == cache.latestVersion {
			t.Fatal("expected latestVersion to update")
		}

	})

	t.Run("Are Available", func(t *testing.T) {
		if !cache.AreAvailable() {
			t.Fatalf("Cache reports not updates but Current is %s and Latest is %s", cache.currentVersion, cache.latestVersion)
		}
	})

	t.Run("Latest Version", func(t *testing.T) {
		gotLatest := cache.LatestVersion()
		if gotLatest != cache.latestVersion {
			t.Fatalf("Malformed latest version. Expected: %s but got: %s", cache.latestVersion, gotLatest)
		}
	})
}

func withVersionServer(t *testing.T, version *string, status *int) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(*status)
		fmt.Fprint(w, *version)
	}))
	t.Cleanup(srv.Close)
	orig := versionCheckURL
	versionCheckURL = srv.URL
	t.Cleanup(func() { versionCheckURL = orig })
}

// Regression: a stale re-check must update the shared cache, not a copy, so
// the Updates page and the badge on other admin pages agree.
func TestUpdatesStaleCheckPersists(t *testing.T) {
	version, status := "v0.17.2\n", http.StatusOK
	withVersionServer(t, &version, &status)

	cache := &updatesCache{frequency: time.Hour, currentVersion: "v0.17.2"}
	if err := cache.CheckNow(); err != nil {
		t.Fatal(err)
	}
	if cache.AreAvailableNoCheck() {
		t.Fatal("expected no update when latest == current")
	}

	// New release ships; cache goes stale.
	version = "v0.17.3"
	cache.lastCheck = time.Now().Add(-2 * time.Hour)
	if !cache.AreAvailable() {
		t.Fatal("expected update available after stale re-check")
	}
	if got := cache.LatestVersion(); got != "v0.17.3" {
		t.Fatalf("latest version = %s, want v0.17.3", got)
	}
	if time.Since(cache.LastCheck()) > time.Minute {
		t.Fatal("lastCheck not updated on shared cache")
	}
	if !cache.AreAvailableNoCheck() {
		t.Fatal("badge check (AreAvailableNoCheck) disagrees with Updates page")
	}
}

func TestUpdatesCheckIfStale(t *testing.T) {
	version, status := "v0.17.3", http.StatusOK
	withVersionServer(t, &version, &status)

	cache := &updatesCache{frequency: time.Hour, currentVersion: "v0.17.2", latestVersion: "v0.17.2"}
	cache.CheckIfStale()
	deadline := time.Now().Add(5 * time.Second)
	for !cache.AreAvailableNoCheck() {
		if time.Now().After(deadline) {
			t.Fatal("background check never updated cache")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestUpdatesCheckErrorClears(t *testing.T) {
	version, status := "v0.17.3", http.StatusInternalServerError
	withVersionServer(t, &version, &status)

	cache := &updatesCache{frequency: time.Hour, currentVersion: "v0.17.2"}
	if err := cache.CheckNow(); err == nil || !cache.CheckFailed() {
		t.Fatal("expected failed check on non-200")
	}
	status = http.StatusOK
	if err := cache.CheckNow(); err != nil || cache.CheckFailed() {
		t.Fatalf("expected check error to clear, got %v", err)
	}

	version = "0.17.4"
	if err := cache.CheckNow(); err != nil || cache.LatestVersion() != "v0.17.4" {
		t.Fatalf("expected missing v prefix to be tolerated, got %v / %s", err, cache.LatestVersion())
	}

	version = "<html>oops</html>"
	if err := cache.CheckNow(); err == nil {
		t.Fatal("expected invalid version body to fail")
	}
	if cache.LatestVersion() != "v0.17.4" {
		t.Fatal("invalid body should not change latest version")
	}
}

func TestReleaseVersion(t *testing.T) {
	cases := map[string]string{
		"v0.17.2":                   "v0.17.2",
		"v0.17.2-17-ga154a12":       "v0.17.2",
		"v0.17.2-3-gdeadbeef-dirty": "v0.17.2",
		"v0.18.0-beta.1":            "v0.18.0-beta.1",
	}
	for in, want := range cases {
		if got := releaseVersion(in); got != want {
			t.Errorf("releaseVersion(%q) = %q, want %q", in, got, want)
		}
	}
	if CompareSemver("v0.17.2", releaseVersion("v0.17.2-17-ga154a12")) != 0 {
		t.Error("post-release dev build should not see its own release as an update")
	}
}
