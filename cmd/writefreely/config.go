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
	cmdConfig cli.Command = cli.Command{
		Name:  "config",
		Usage: "config management tools",
		Subcommands: []*cli.Command{
			&cmdConfigGenerate,
			&cmdConfigInteractive,
		},
	}

	cmdConfigGenerate cli.Command = cli.Command{
		Name:    "generate",
		Aliases: []string{"gen"},
		Usage:   "Generate a basic configuration",
		Action:  genConfigAction,
	}

	cmdConfigInteractive cli.Command = cli.Command{
		Name:   "start",
		Usage:  "Interactive configuration process",
		Action: interactiveConfigAction,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "sections",
				Value: "server db app",
				Usage: "Which sections of the configuration to go through\n" +
					"valid values of sections flag are any combination of 'server', 'db' and 'app' \n" +
					"example: writefreely config start --sections \"db app\"",
			},
		},
	}
)

func genConfigAction(c *cli.Context) error {
	app := writefreely.NewApp(c.String("c"))
	return writefreely.CreateConfig(app)
}

func interactiveConfigAction(c *cli.Context) error {
	app := writefreely.NewApp(c.String("c"))
	writefreely.DoConfig(app, c.String("sections"))
	return nil
}
