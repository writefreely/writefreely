/*
 * Copyright © 2018 Musing Studio LLC.
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
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/manifoldco/promptui"
	"github.com/mitchellh/go-wordwrap"
	"github.com/writeas/web-core/auth"
)

type SetupData struct {
	User   *UserCreation
	Config *Config
}

func Configure(fname string, configSections string) (*SetupData, error) {
	data := &SetupData{}
	var err error
	if fname == "" {
		fname = FileName
	}

	data.Config, err = Load(fname)
	var action string
	isNewCfg := false
	if err != nil {
		fmt.Printf("No %s configuration yet. Creating new.\n", fname)
		data.Config = New()
		action = "generate"
		isNewCfg = true
	} else {
		fmt.Printf("Loaded configuration %s.\n", fname)
		action = "update"
	}
	title := color.New(color.Bold, color.BgGreen).PrintFunc()

	intro := color.New(color.Bold, color.FgWhite).PrintlnFunc()
	fmt.Println()
	intro("  ✍ WriteFreely Configuration ✍")
	fmt.Println()
	fmt.Println(wordwrap.WrapString("  This quick configuration process will "+action+" the application's config file, "+fname+".\n\n  It validates your input along the way, so you can be sure any future errors aren't caused by a bad configuration. If you'd rather configure your server manually, instead run: writefreely --create-config and edit that file.", 75))
	fmt.Println()

	tmpls := &promptui.PromptTemplates{
		Success: "{{ . | bold | faint }}: ",
	}
	selTmpls := &promptui.SelectTemplates{
		Selected: `{{.Label}} {{ . | faint }}`,
	}

	var selPrompt promptui.Select
	var prompt promptui.Prompt

	if strings.Contains(configSections, "server") {
		title(" Server setup ")
		fmt.Println()

		// Environment selection
		selPrompt = promptui.Select{
			Templates: selTmpls,
			Label:     "Environment",
			Items:     []string{"Development", "Production, standalone", "Production, behind reverse proxy"},
		}
		_, envType, err := selPrompt.Run()
		if err != nil {
			return data, err
		}
		isDevEnv := envType == "Development"
		isStandalone := envType == "Production, standalone"

		data.Config.Server.Dev = isDevEnv

		if isDevEnv || !isStandalone {
			// Running in dev environment or behind reverse proxy; ask for port
			prompt = promptui.Prompt{
				Templates: tmpls,
				Label:     "Local port",
				Validate:  validatePort,
				Default:   fmt.Sprintf("%d", data.Config.Server.Port),
			}
			port, err := prompt.Run()
			if err != nil {
				return data, err
			}
			data.Config.Server.Port, _ = strconv.Atoi(port) // Ignore error, as we've already validated number
		}

		if isStandalone {
			selPrompt = promptui.Select{
				Templates: selTmpls,
				Label:     "Web server mode",
				Items:     []string{"Insecure (port 80)", "Secure (port 443), manual certificate", "Secure (port 443), auto certificate"},
			}
			sel, _, err := selPrompt.Run()
			if err != nil {
				return data, err
			}
			if sel == 0 {
				data.Config.Server.Autocert = false
				data.Config.Server.Port = 80
				data.Config.Server.TLSCertPath = ""
				data.Config.Server.TLSKeyPath = ""
			} else if sel == 1 || sel == 2 {
				data.Config.Server.Port = 443
				data.Config.Server.Autocert = sel == 2

				if sel == 1 {
					// Manual certificate configuration
					prompt = promptui.Prompt{
						Templates: tmpls,
						Label:     "Certificate path",
						Validate:  validateNonEmpty,
						Default:   data.Config.Server.TLSCertPath,
					}
					data.Config.Server.TLSCertPath, err = prompt.Run()
					if err != nil {
						return data, err
					}

					prompt = promptui.Prompt{
						Templates: tmpls,
						Label:     "Key path",
						Validate:  validateNonEmpty,
						Default:   data.Config.Server.TLSKeyPath,
					}
					data.Config.Server.TLSKeyPath, err = prompt.Run()
					if err != nil {
						return data, err
					}
				} else {
					// Automatic certificate
					data.Config.Server.TLSCertPath = "certs"
					data.Config.Server.TLSKeyPath = "certs"
				}
			}
		} else {
			data.Config.Server.TLSCertPath = ""
			data.Config.Server.TLSKeyPath = ""
		}

		fmt.Println()
	}

	if strings.Contains(configSections, "db") {
		title(" Database setup ")
		fmt.Println()

		selPrompt = promptui.Select{
			Templates: selTmpls,
			Label:     "Database driver",
			Items:     []string{"MySQL", "SQLite"},
		}
		sel, _, err := selPrompt.Run()
		if err != nil {
			return data, err
		}

		if sel == 0 {
			// Configure for MySQL
			data.Config.UseMySQL(isNewCfg)

			prompt = promptui.Prompt{
				Templates: tmpls,
				Label:     "Username",
				Validate:  validateNonEmpty,
				Default:   data.Config.Database.User,
			}
			data.Config.Database.User, err = prompt.Run()
			if err != nil {
				return data, err
			}

			prompt = promptui.Prompt{
				Templates: tmpls,
				Label:     "Password",
				Validate:  validateNonEmpty,
				Default:   data.Config.Database.Password,
				Mask:      '*',
			}
			data.Config.Database.Password, err = prompt.Run()
			if err != nil {
				return data, err
			}

			prompt = promptui.Prompt{
				Templates: tmpls,
				Label:     "Database name",
				Validate:  validateNonEmpty,
				Default:   data.Config.Database.Database,
			}
			data.Config.Database.Database, err = prompt.Run()
			if err != nil {
				return data, err
			}

			prompt = promptui.Prompt{
				Templates: tmpls,
				Label:     "Host",
				Validate:  validateNonEmpty,
				Default:   data.Config.Database.Host,
			}
			data.Config.Database.Host, err = prompt.Run()
			if err != nil {
				return data, err
			}

			prompt = promptui.Prompt{
				Templates: tmpls,
				Label:     "Port",
				Validate:  validatePort,
				Default:   fmt.Sprintf("%d", data.Config.Database.Port),
			}
			dbPort, err := prompt.Run()
			if err != nil {
				return data, err
			}
			data.Config.Database.Port, _ = strconv.Atoi(dbPort) // Ignore error, as we've already validated number

			selPrompt = promptui.Select{
				Templates: selTmpls,
				Label:     "Are you using MySQL 8.0.4 or higher?",
				Items:     []string{"Yes", "No"},
			}
			_, icuRegex, err := selPrompt.Run()
			if err != nil {
				return data, err
			}
			data.Config.Database.IcuRegex = icuRegex == "Yes"
		} else if sel == 1 {
			// Configure for SQLite
			data.Config.UseSQLite(isNewCfg)

			prompt = promptui.Prompt{
				Templates: tmpls,
				Label:     "Filename",
				Validate:  validateNonEmpty,
				Default:   data.Config.Database.FileName,
			}
			data.Config.Database.FileName, err = prompt.Run()
			if err != nil {
				return data, err
			}
		}

		fmt.Println()
	}

	if strings.Contains(configSections, "app") {
		title(" App setup ")
		fmt.Println()

		selPrompt = promptui.Select{
			Templates: selTmpls,
			Label:     "Site type",
			Items:     []string{"Single user blog", "Multi-user instance"},
		}
		_, usersType, err := selPrompt.Run()
		if err != nil {
			return data, err
		}
		data.Config.App.SingleUser = usersType == "Single user blog"

		if data.Config.App.SingleUser {
			data.User = &UserCreation{}

			//   prompt for username
			prompt = promptui.Prompt{
				Templates: tmpls,
				Label:     "Admin username",
				Validate:  validateNonEmpty,
			}
			data.User.Username, err = prompt.Run()
			if err != nil {
				return data, err
			}

			//   prompt for password
			prompt = promptui.Prompt{
				Templates: tmpls,
				Label:     "Admin password",
				Validate:  validateNonEmpty,
			}
			newUserPass, err := prompt.Run()
			if err != nil {
				return data, err
			}

			data.User.HashedPass, err = auth.HashPass([]byte(newUserPass))
			if err != nil {
				return data, err
			}
		}

		siteNameLabel := "Instance name"
		if data.Config.App.SingleUser {
			siteNameLabel = "Blog name"
		}
		prompt = promptui.Prompt{
			Templates: tmpls,
			Label:     siteNameLabel,
			Validate:  validateNonEmpty,
			Default:   data.Config.App.SiteName,
		}
		data.Config.App.SiteName, err = prompt.Run()
		if err != nil {
			return data, err
		}

		prompt = promptui.Prompt{
			Templates: tmpls,
			Label:     "Public URL",
			Validate:  validateDomain,
			Default:   data.Config.App.Host,
		}
		data.Config.App.Host, err = prompt.Run()
		if err != nil {
			return data, err
		}

		if !data.Config.App.SingleUser {
			selPrompt = promptui.Select{
				Templates: selTmpls,
				Label:     "Registration",
				Items:     []string{"Open", "Closed"},
			}
			_, regType, err := selPrompt.Run()
			if err != nil {
				return data, err
			}
			data.Config.App.OpenRegistration = regType == "Open"

			prompt = promptui.Prompt{
				Templates: tmpls,
				Label:     "Max blogs per user",
				Default:   fmt.Sprintf("%d", data.Config.App.MaxBlogs),
			}
			maxBlogs, err := prompt.Run()
			if err != nil {
				return data, err
			}
			data.Config.App.MaxBlogs, _ = strconv.Atoi(maxBlogs) // Ignore error, as we've already validated number
		}

		selPrompt = promptui.Select{
			Templates: selTmpls,
			Label:     "Federation",
			Items:     []string{"Enabled", "Disabled"},
		}
		_, fedType, err := selPrompt.Run()
		if err != nil {
			return data, err
		}
		data.Config.App.Federation = fedType == "Enabled"

		if data.Config.App.Federation {
			selPrompt = promptui.Select{
				Templates: selTmpls,
				Label:     "Usage stats (active users, posts)",
				Items:     []string{"Public", "Private"},
			}
			_, fedStatsType, err := selPrompt.Run()
			if err != nil {
				return data, err
			}
			data.Config.App.PublicStats = fedStatsType == "Public"

			selPrompt = promptui.Select{
				Templates: selTmpls,
				Label:     "Instance metadata privacy",
				Items:     []string{"Public", "Private"},
			}
			_, fedStatsType, err = selPrompt.Run()
			if err != nil {
				return data, err
			}
			data.Config.App.Private = fedStatsType == "Private"
		}
	}

	return data, Save(data.Config, fname)
}
