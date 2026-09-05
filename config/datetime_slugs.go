package config

import (
	"fmt"
	"strings"
	"time"
	_ "time/tzdata" // Keep IANA zones available in standalone binaries and minimal images.
)

func (ac *AppCfg) initDatetimeSlugs() error {
	ac.DatetimeSlugTimezone = strings.TrimSpace(ac.DatetimeSlugTimezone)
	if ac.DatetimeSlugTimezone == "" {
		ac.DatetimeSlugTimezone = "UTC"
	}
	if !ac.DatetimeSlugs {
		return nil
	}
	loc, err := ac.slugLocation()
	if err != nil {
		return err
	}
	ac.datetimeSlugLocation = loc
	return nil
}

func (ac *AppCfg) slugLocation() (*time.Location, error) {
	name := strings.TrimSpace(ac.DatetimeSlugTimezone)
	if name == "" {
		name = "UTC"
	}
	if name == "Local" {
		return nil, fmt.Errorf("app.datetime_slug_timezone must be UTC or an IANA timezone, not Local")
	}
	if ac.datetimeSlugLocation != nil && ac.datetimeSlugLocation.String() == name {
		return ac.datetimeSlugLocation, nil
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("invalid app.datetime_slug_timezone %q: %w", name, err)
	}
	return loc, nil
}

// DatetimeSlug formats the publication instant without changing it. An empty
// result means the caller should use the original automatic slug generation.
func (ac *AppCfg) DatetimeSlug(created time.Time) (string, error) {
	if !ac.DatetimeSlugs {
		return "", nil
	}
	loc, err := ac.slugLocation()
	if err != nil {
		return "", err
	}
	return created.In(loc).Format("20060102150405"), nil
}
