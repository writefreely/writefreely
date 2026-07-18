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
	"time"

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

	cmdResetPass cli.Command = cli.Command{
		Name:    "reset-pass",
		Usage:   "Reset user's password",
		Aliases: []string{"resetpass", "reset"},
		Action:  resetPassAction,
	}

	cmdUsers cli.Command = cli.Command{
		Name:  "users",
		Usage: "mass user moderation tools",
		Subcommands: []*cli.Command{
			&cmdListUsers,
			&cmdSilenceUsers,
			&cmdDeleteUsers,
		},
	}

	cmdListUsers cli.Command = cli.Command{
		Name:    "list",
		Usage:   "List users matching filters",
		Aliases: []string{"ls"},
		Flags:   userFilterFlags(),
		Action:  listUsersAction,
	}

	cmdSilenceUsers cli.Command = cli.Command{
		Name:   "silence",
		Usage:  "Silence all users matching filters, with confirmation",
		Flags:  userFilterFlags(),
		Action: silenceUsersAction,
	}

	cmdDeleteUsers cli.Command = cli.Command{
		Name:   "delete",
		Usage:  "Delete all users matching filters and their content, with confirmation",
		Flags:  userFilterFlags(),
		Action: deleteUsersAction,
	}
)

// userFilterFlags returns the shared filter flags used by the `users`
// subcommands (list/silence/delete). A fresh slice is returned per call so the
// same flag definition is the single source of truth without sharing flag
// state across sibling commands.
func userFilterFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:  "since",
			Usage: "Only users created on/after this date (YYYY-MM-DD)",
		},
		&cli.StringFlag{
			Name:  "until",
			Usage: "Only users created before this date (YYYY-MM-DD)",
		},
		&cli.BoolFlag{
			Name:  "no-invite",
			Usage: "Only users who signed up without an invite code",
		},
		&cli.BoolFlag{
			Name:  "no-oauth",
			Usage: "Only users who did not register via an OAuth provider",
		},
		&cli.IntFlag{
			Name:  "max-posts",
			Value: -1,
			Usage: "Only users with at most N posts (-1 = no limit)",
		},
	}
}

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

// userFilterFromContext builds a UserFilter from the shared filter flags,
// returning a clear error on malformed dates.
func userFilterFromContext(c *cli.Context) (writefreely.UserFilter, error) {
	const dateFmt = "2006-01-02"
	filter := writefreely.UserFilter{
		NoInvite: c.Bool("no-invite"),
		NoOAuth:  c.Bool("no-oauth"),
		MaxPosts: c.Int("max-posts"),
	}
	if s := c.String("since"); s != "" {
		t, err := time.Parse(dateFmt, s)
		if err != nil {
			return filter, fmt.Errorf("invalid --since date %q, expected YYYY-MM-DD", s)
		}
		filter.Since = &t
	}
	if s := c.String("until"); s != "" {
		t, err := time.Parse(dateFmt, s)
		if err != nil {
			return filter, fmt.Errorf("invalid --until date %q, expected YYYY-MM-DD", s)
		}
		filter.Until = &t
	}
	return filter, nil
}

func moderateUsersAction(c *cli.Context, action writefreely.UserAction) error {
	filter, err := userFilterFromContext(c)
	if err != nil {
		return err
	}
	app := writefreely.NewApp(c.String("c"))
	return writefreely.ModerateUsers(app, filter, action)
}

func listUsersAction(c *cli.Context) error {
	return moderateUsersAction(c, writefreely.ActionList)
}

func silenceUsersAction(c *cli.Context) error {
	return moderateUsersAction(c, writefreely.ActionSilence)
}

func deleteUsersAction(c *cli.Context) error {
	return moderateUsersAction(c, writefreely.ActionDelete)
}
