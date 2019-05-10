/*
 * Copyright © 2018-2019 A Bunch Tell LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package main

import (
	"flag"
	"github.com/writeas/web-core/log"
	"github.com/writeas/writefreely"
	"os"
)

func main() {
	// General options usable with other commands
	debugPtr := flag.Bool("debug", false, "Enables debug logging.")
	configFile := flag.String("c", "config.ini", "The configuration file to use")

	// Setup actions
	createConfig := flag.Bool("create-config", false, "Creates a basic configuration and exits")
	doConfig := flag.Bool("config", false, "Run the configuration process")
	genKeys := flag.Bool("gen-keys", false, "Generate encryption and authentication keys")
	createSchema := flag.Bool("init-db", false, "Initialize app database")
	migrate := flag.Bool("migrate", false, "Migrate the database")

	// Admin actions
	createAdmin := flag.String("create-admin", "", "Create an admin with the given username:password")
	createUser := flag.String("create-user", "", "Create a regular user with the given username:password")
	resetPassUser := flag.String("reset-pass", "", "Reset the given user's password")
	outputVersion := flag.Bool("v", false, "Output the current version")
	flag.Parse()

	app := writefreely.NewApp(*configFile)

	if *outputVersion {
		writefreely.OutputVersion()
		os.Exit(0)
	} else if *createConfig {
		err := writefreely.CreateConfig(app)
		if err != nil {
			log.Error(err.Error())
			os.Exit(1)
		}
		os.Exit(0)
	} else if *doConfig {
		writefreely.DoConfig(app)
		os.Exit(0)
	} else if *genKeys {
		err := writefreely.GenerateKeys(app)
		if err != nil {
			log.Error(err.Error())
			os.Exit(1)
		}
		os.Exit(0)
	} else if *createSchema {
		err := writefreely.CreateSchema(app)
		if err != nil {
			log.Error(err.Error())
			os.Exit(1)
		}
		os.Exit(0)
	} else if *createAdmin != "" {
		err := writefreely.CreateUser(app, *createAdmin, true)
		if err != nil {
			log.Error(err.Error())
			os.Exit(1)
		}
		os.Exit(0)
	} else if *createUser != "" {
		err := writefreely.CreateUser(app, *createUser, false)
		if err != nil {
			log.Error(err.Error())
			os.Exit(1)
		}
		os.Exit(0)
	} else if *resetPassUser != "" {
		err := writefreely.ResetPassword(app, *resetPassUser)
		if err != nil {
			log.Error(err.Error())
			os.Exit(1)
		}
		os.Exit(0)
	} else if *migrate {
		err := writefreely.Migrate(app)
		if err != nil {
			log.Error(err.Error())
			os.Exit(1)
		}
		os.Exit(0)
	}

	writefreely.Serve(app, *debugPtr)
}
