package config

import (
	"strings"
)

// FriendlyHost returns the app's Host sans any schema
func (ac AppCfg) FriendlyHost() string {
	return ac.Host[strings.Index(ac.Host, "://")+len("://"):]
}

func (ac AppCfg) CanCreateBlogs(currentlyUsed uint64) bool {
	if ac.MaxBlogs <= 0 {
		return true
	}
	return int(currentlyUsed) < ac.MaxBlogs
}
