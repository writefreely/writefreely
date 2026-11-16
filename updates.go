/*
 * Copyright © 2019-2020 Musing Studio LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package writefreely

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/writeas/web-core/log"
)

// updatesCacheTime is the default interval between cache updates for new
// software versions
const defaultUpdatesCacheTime = 12 * time.Hour

var versionCheckURL = "https://version.writefreely.org"

// gitDescribeSuffix matches the suffix `git describe` appends to a version
// built from commits after a tag, e.g. the "-17-ga154a12" in
// "0.17.1-17-ga154a12".
var gitDescribeSuffix = regexp.MustCompile(`-\d+-g[0-9a-f]+(-dirty)?$`)

// updatesCache holds data about current and new releases of the writefreely
// software
type updatesCache struct {
	mu             sync.RWMutex
	frequency      time.Duration
	lastCheck      time.Time
	latestVersion  string
	currentVersion string
	checkError     error
	checking       bool
}

// CheckNow asks for the latest released version of writefreely and updates
// the cache last checked time. If the version postdates the current 'latest'
// the version value is replaced.
func (uc *updatesCache) CheckNow() error {
	if debugging {
		log.Info("[update check] Checking for update now.")
	}
	latestRemote, err := newVersionCheck()
	uc.mu.Lock()
	defer uc.mu.Unlock()
	uc.lastCheck = time.Now()
	if err != nil {
		log.Error("[update check] Failed: %v", err)
		uc.checkError = err
		return err
	}
	uc.checkError = nil
	if CompareSemver(latestRemote, uc.latestVersion) == 1 {
		uc.latestVersion = latestRemote
	}
	return nil
}

// checkInBackground runs CheckNow in a goroutine, marking the cache as having
// a check in flight so concurrent callers don't start another.
func (uc *updatesCache) checkInBackground() {
	uc.mu.Lock()
	if uc.checking {
		uc.mu.Unlock()
		return
	}
	uc.checking = true
	uc.mu.Unlock()
	go func() {
		uc.CheckNow()
		uc.mu.Lock()
		uc.checking = false
		uc.mu.Unlock()
	}()
}

// CheckIfStale triggers a background check if the frequency duration has
// passed since the last check, without blocking the caller.
func (uc *updatesCache) CheckIfStale() {
	uc.mu.RLock()
	stale := time.Since(uc.lastCheck) > uc.frequency
	uc.mu.RUnlock()
	if stale {
		uc.checkInBackground()
	}
}

// AreAvailable updates the cache if the frequency duration has passed
// then returns if the latest release is newer than the current running version.
func (uc *updatesCache) AreAvailable() bool {
	uc.mu.RLock()
	stale := time.Since(uc.lastCheck) > uc.frequency
	uc.mu.RUnlock()
	if stale {
		uc.CheckNow()
	}
	return uc.AreAvailableNoCheck()
}

// AreAvailableNoCheck returns if the latest release is newer than the current
// running version.
func (uc *updatesCache) AreAvailableNoCheck() bool {
	uc.mu.RLock()
	defer uc.mu.RUnlock()
	return CompareSemver(uc.latestVersion, uc.currentVersion) == 1
}

// LatestVersion returns the latest stored version available.
func (uc *updatesCache) LatestVersion() string {
	uc.mu.RLock()
	defer uc.mu.RUnlock()
	return uc.latestVersion
}

// LastCheck returns the time of the last update check.
func (uc *updatesCache) LastCheck() time.Time {
	uc.mu.RLock()
	defer uc.mu.RUnlock()
	return uc.lastCheck
}

// CheckFailed returns whether the most recent update check failed.
func (uc *updatesCache) CheckFailed() bool {
	uc.mu.RLock()
	defer uc.mu.RUnlock()
	return uc.checkError != nil
}

func (uc *updatesCache) ReleaseURL() string {
	return "https://writefreely.org/releases/" + uc.LatestVersion()
}

// ReleaseNotesURL returns the full URL to the blog.writefreely.org release notes
// for the latest version as stored in the cache.
func (uc *updatesCache) ReleaseNotesURL() string {
	return wfReleaseNotesURL(uc.LatestVersion())
}

func wfReleaseNotesURL(v string) string {
	ver := strings.TrimPrefix(v, "v")
	ver = strings.TrimSuffix(ver, ".0")
	// hack until go 1.12 in build/travis
	seg := strings.Split(ver, ".")
	return "https://blog.writefreely.org/version-" + strings.Join(seg, "-")
}

// newUpdatesCache returns an initialized updates cache
func newUpdatesCache(expiry time.Duration) *updatesCache {
	cache := &updatesCache{
		frequency:      expiry,
		currentVersion: releaseVersion("v" + softwareVer),
	}
	cache.checkInBackground()
	return cache
}

// releaseVersion strips any `git describe` suffix from the given version, so
// that a build made from commits after a release tag compares as that release
// rather than as a pre-release of it.
func releaseVersion(v string) string {
	return gitDescribeSuffix.ReplaceAllString(v, "")
}

// InitUpdates initializes the updates cache, if the config value is set
// It uses the defaultUpdatesCacheTime for the cache expiry
func (app *App) InitUpdates() {
	if app.cfg.App.UpdateChecks {
		app.updates = newUpdatesCache(defaultUpdatesCacheTime)
	}
}

var versionCheckClient = &http.Client{Timeout: 10 * time.Second}

func newVersionCheck() (string, error) {
	if debugging {
		log.Info("[update check] GET %s", versionCheckURL)
	}
	res, err := versionCheckClient.Get(versionCheckURL)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d from %s", res.StatusCode, versionCheckURL)
	}

	body, err := io.ReadAll(io.LimitReader(res.Body, 64))
	if err != nil {
		return "", err
	}
	v := strings.TrimSpace(string(body))
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	if !IsValid(v) {
		return "", fmt.Errorf("invalid version %q from %s", v, versionCheckURL)
	}
	return v, nil
}
