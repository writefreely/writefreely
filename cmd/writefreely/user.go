/*
 * Copyright © 2020 A Bunch Tell LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package main

import (
	"github.com/writeas/writefreely"

	"github.com/urfave/cli/v2"
)

var (
	cmdUser cli.Command = cli.Command{
		Name:  "user",
		Usage: "user management tools",
		Subcommands: []*cli.Command{
			&cmdAddUser,
			&cmdDelUser,
			&cmdResetPass,
			// TODO: possibly add a user list command
		},
	}

	cmdAddUser cli.Command = cli.Command{
		Name:    "add",
		Usage:   "Add new user",
		Aliases: []string{"a"},
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "admin",
				Value: false,
				Usage: "Create admin user",
			},
		},
		Action: addUserAction,
	}

	cmdDelUser cli.Command = cli.Command{
		Name:    "delete",
		Usage:   "Delete user",
		Aliases: []string{"del", "d"},
		Action:  delUserAction,
	}

	cmdResetPass cli.Command = cli.Command{
		Name:    "reset-pass",
		Usage:   "Reset user's password",
		Aliases: []string{"resetpass", "reset"},
		Action:  resetPassAction,
	}
)

func addUserAction(c *cli.Context) error {
	credentials := ""
	if c.NArg() > 0 {
		credentials = c.Args().Get(0)
	}
	username, password, err := parseCredentials(credentials)
	if err != nil {
		return err
	}
	app := writefreely.NewApp(c.String("c"))
	return writefreely.CreateUser(app, username, password, c.Bool("admin"))
}

func delUserAction(c *cli.Context) error {
	username := ""
	if c.NArg() > 0 {
		username = c.Args().Get(0)
	}
	app := writefreely.NewApp(c.String("c"))
	return writefreely.DoDeleteAccount(app, username)
}

func resetPassAction(c *cli.Context) error {
	username := ""
	if c.NArg() > 0 {
		username = c.Args().Get(0)
	}
	app := writefreely.NewApp(c.String("c"))
	return writefreely.ResetPassword(app, username)
}
