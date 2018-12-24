/*
 * Copyright © 2018 A Bunch Tell LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */
package config

import (
	"fmt"
	"regexp"
	"strconv"
)

var (
	domainReg = regexp.MustCompile("^https?://")
)

const (
	minPort = 80
	maxPort = 1<<16 - 1
)

func validateDomain(i string) error {
	if !domainReg.MatchString(i) {
		return fmt.Errorf("Domain must start with http:// or https://")
	}
	return nil
}

func validatePort(i string) error {
	p, err := strconv.Atoi(i)
	if err != nil {
		return err
	}
	if p < minPort || p > maxPort {
		return fmt.Errorf("Port must be a number %d - %d", minPort, maxPort)
	}
	return nil
}

func validateNonEmpty(i string) error {
	if i == "" {
		return fmt.Errorf("Must not be empty")
	}
	return nil
}
