/*
 * Copyright © 2018-2019, 2021 Musing Studio LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

// package page provides mechanisms and data for generating a WriteFreely page.
package page

import (
	"github.com/writefreely/writefreely/config"
	"strings"
)

type StaticPage struct {
	// App configuration
	config.AppCfg
	Version   string
	HeaderNav bool
	CustomCSS bool

	// Request values
	Path          string
	Username      string
	Values        map[string]string
	Flashes       []string
	CanViewReader bool
	IsAdmin       bool
	CanInvite     bool

    Locales   string
    Tr  func (str string, ParamsToTranslate ...interface{}) interface{}
}

// SanitizeHost alters the StaticPage to contain a real hostname. This is
// especially important for the Tor hidden service, as it can be served over
// proxies, messing up the apparent hostname.
func (sp *StaticPage) SanitizeHost(cfg *config.Config) {
	if cfg.Server.HiddenHost != "" && strings.HasPrefix(sp.Host, cfg.Server.HiddenHost) {
		sp.Host = cfg.Server.HiddenHost
	}
}

func (sp StaticPage) OfficialVersion() string {
	p := strings.Split(sp.Version, "-")
	return p[0]
}
