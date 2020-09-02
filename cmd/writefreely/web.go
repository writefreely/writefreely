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
	"github.com/writeas/web-core/log"
	"github.com/writeas/writefreely"

	"github.com/gorilla/mux"
	"github.com/urfave/cli/v2"
)

var (
	cmdServe cli.Command = cli.Command{
		Name:    "serve",
		Aliases: []string{"web"},
		Usage:   "Run web application",
		Action:  serveAction,
	}
)

func serveAction(c *cli.Context) error {
	// Initialize the application
	app := writefreely.NewApp(c.String("c"))
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
