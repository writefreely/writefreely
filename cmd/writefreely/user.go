/*
 * Copyright © 2020-2021 Musing Studio LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package main

import (
	"fmt"

	"github.com/urfave/cli/v2"
	"github.com/writefreely/writefreely"
)

var (
	cmdUser cli.Command = cli.Command{
		Name:  "user",
		Usage: "user management tools",
		Subcommands: []*cli.Command{
			&cmdAddUser,
			&cmdDelUser,
			&cmdSilenceUser,
			&cmdResetPass,
			// TODO: possibly add a user list command
		},
	}

	cmdAddUser cli.Command = cli.Command{
		Name:    "create",
		Usage:   "Add new user",
		Aliases: []string{"a", "add"},
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

	cmdSilenceUser cli.Command = cli.Command{
		Name:    "silence",
		Usage:   "Silence user",
		Aliases: []string{"sil", "s"},
		Action:  silenceUserAction,
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
	} else {
		return fmt.Errorf("No user passed. Example: writefreely user add [USER]:[PASSWORD]")
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
	} else {
		return fmt.Errorf("No user passed. Example: writefreely user delete [USER]")
	}
	app := writefreely.NewApp(c.String("c"))
	return writefreely.DoDeleteAccount(app, username)
}

func silenceUserAction(c *cli.Context) error {
	username := ""
	if c.NArg() > 0 {
		username = c.Args().Get(0)
	} else {
		return fmt.Errorf("No user passed. Example: writefreely user silence [USER]")
	}
	app := writefreely.NewApp(c.String("c"))
	return writefreely.DoSilenceAccount(app, username)
}

func resetPassAction(c *cli.Context) error {
	username := ""
	if c.NArg() > 0 {
		username = c.Args().Get(0)
	} else {
		return fmt.Errorf("No user passed. Example: writefreely user reset-pass [USER]")
	}
	app := writefreely.NewApp(c.String("c"))
	return writefreely.ResetPassword(app, username)
}
