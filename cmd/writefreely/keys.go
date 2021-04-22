/*
 * Copyright © 2020-2021 A Bunch Tell LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package main

import (
	"github.com/urfave/cli/v2"
	"github.com/writefreely/writefreely"
)

var (
	cmdKeys cli.Command = cli.Command{
		Name:  "keys",
		Usage: "key management tools",
		Subcommands: []*cli.Command{
			&cmdGenerateKeys,
		},
	}

	cmdGenerateKeys cli.Command = cli.Command{
		Name:    "generate",
		Aliases: []string{"gen"},
		Usage:   "Generate encryption and authentication keys",
		Action:  genKeysAction,
	}
)

func genKeysAction(c *cli.Context) error {
	app := writefreely.NewApp(c.String("c"))
	return writefreely.GenerateKeyFiles(app)
}
