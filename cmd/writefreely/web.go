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
	"github.com/gorilla/mux"
	"github.com/urfave/cli/v2"
	"github.com/writeas/web-core/log"
	"github.com/writefreely/writefreely"
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
	appRouter := mux.NewRouter()
	app.InitStaticRoutes(appRouter)
	writefreely.InitRoutes(app, appRouter)
	writefreely.MountSubdirectory(app, r, appRouter)

	// Serve the application
	writefreely.Serve(app, r)

	return nil
}
