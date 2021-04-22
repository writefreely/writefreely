/*
 * Copyright © 2018-2021 A Bunch Tell LLC.
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
	"os"
	"strings"

	"github.com/gorilla/mux"
	"github.com/urfave/cli/v2"
	"github.com/writeas/web-core/log"
	"github.com/writefreely/writefreely"
)

func main() {
	cli.VersionPrinter = func(c *cli.Context) {
		fmt.Printf("%s\n", c.App.Version)
	}
	app := &cli.App{
		Name:    "WriteFreely",
		Usage:   "A beautifully pared-down blogging platform",
		Version: writefreely.FormatVersion(),
		Action:  legacyActions, // legacy due to use of flags for switching actions
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:   "create-config",
				Value:  false,
				Usage:  "Generate a basic configuration",
				Hidden: true,
			},
			&cli.BoolFlag{
				Name:   "config",
				Value:  false,
				Usage:  "Interactive configuration process",
				Hidden: true,
			},
			&cli.StringFlag{
				Name:  "sections",
				Value: "server db app",
				Usage: "Which sections of the configuration to go through (requires --config)\n" +
					"valid values are any combination of 'server', 'db' and 'app' \n" +
					"example: writefreely --config --sections \"db app\"",
				Hidden: true,
			},
			&cli.BoolFlag{
				Name:   "gen-keys",
				Value:  false,
				Usage:  "Generate encryption and authentication keys",
				Hidden: true,
			},
			&cli.BoolFlag{
				Name:   "init-db",
				Value:  false,
				Usage:  "Initialize app database",
				Hidden: true,
			},
			&cli.BoolFlag{
				Name:   "migrate",
				Value:  false,
				Usage:  "Migrate the database",
				Hidden: true,
			},
			&cli.StringFlag{
				Name:   "create-admin",
				Usage:  "Create an admin with the given username:password",
				Hidden: true,
			},
			&cli.StringFlag{
				Name:   "create-user",
				Usage:  "Create a regular user with the given username:password",
				Hidden: true,
			},
			&cli.StringFlag{
				Name:   "delete-user",
				Usage:  "Delete a user with the given username",
				Hidden: true,
			},
			&cli.StringFlag{
				Name:   "reset-pass",
				Usage:  "Reset the given user's password",
				Hidden: true,
			},
		}, // legacy flags (set to hidden to eventually switch to bash-complete compatible format)
	}

	defaultFlags := []cli.Flag{
		&cli.StringFlag{
			Name:  "c",
			Value: "config.ini",
			Usage: "Load configuration from `FILE`",
		},
		&cli.BoolFlag{
			Name:  "debug",
			Value: false,
			Usage: "Enables debug logging",
		},
	}

	app.Flags = append(app.Flags, defaultFlags...)

	app.Commands = []*cli.Command{
		&cmdUser,
		&cmdDB,
		&cmdConfig,
		&cmdKeys,
		&cmdServe,
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}
}

func legacyActions(c *cli.Context) error {
	app := writefreely.NewApp(c.String("c"))

	switch true {
	case c.IsSet("create-config"):
		return writefreely.CreateConfig(app)
	case c.IsSet("config"):
		writefreely.DoConfig(app, c.String("sections"))
		return nil
	case c.IsSet("gen-keys"):
		return writefreely.GenerateKeyFiles(app)
	case c.IsSet("init-db"):
		return writefreely.CreateSchema(app)
	case c.IsSet("migrate"):
		return writefreely.Migrate(app)
	case c.IsSet("create-admin"):
		username, password, err := parseCredentials(c.String("create-admin"))
		if err != nil {
			return err
		}
		return writefreely.CreateUser(app, username, password, true)
	case c.IsSet("create-user"):
		username, password, err := parseCredentials(c.String("create-user"))
		if err != nil {
			return err
		}
		return writefreely.CreateUser(app, username, password, false)
	case c.IsSet("delete-user"):
		return writefreely.DoDeleteAccount(app, c.String("delete-user"))
	case c.IsSet("reset-pass"):
		return writefreely.ResetPassword(app, c.String("reset-pass"))
	}

	// Initialize the application
	var err error
	log.Info("Starting %s...", writefreely.FormatVersion())
	app, err = writefreely.Initialize(app, c.Bool("debug"))
	if err != nil {
		return err
	}

	// Set app routes
	r := mux.NewRouter()
	writefreely.InitRoutes(app, r)
	app.InitStaticRoutes(r)

	// Serve the application
	writefreely.Serve(app, r)

	return nil
}

func parseCredentials(credentialString string) (string, string, error) {
	creds := strings.Split(credentialString, ":")
	if len(creds) != 2 {
		return "", "", fmt.Errorf("invalid format for passed credentials, must be username:password")
	}
	return creds[0], creds[1], nil
}
