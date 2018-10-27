package config

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/manifoldco/promptui"
	"github.com/mitchellh/go-wordwrap"
	"strconv"
)

func Configure() error {
	c, err := Load()
	if err != nil {
		fmt.Println("No configuration yet. Creating new.")
		c = New()
	} else {
		fmt.Println("Configuration loaded.")
	}
	title := color.New(color.Bold, color.BgGreen).PrintlnFunc()

	intro := color.New(color.Bold, color.FgWhite).PrintlnFunc()
	fmt.Println()
	intro("  ✍ Write Freely Configuration ✍")
	fmt.Println()
	fmt.Println(wordwrap.WrapString("  This quick configuration process will generate the application's config file, "+FileName+".\n\n  It validates your input along the way, so you can be sure any future errors aren't caused by a bad configuration. If you'd rather configure your server manually, instead run: writefreely --create-config and edit that file.", 75))
	fmt.Println()

	title(" Server setup ")
	fmt.Println()

	prompt := promptui.Prompt{
		Label:    "Local port",
		Validate: validatePort,
		Default:  fmt.Sprintf("%d", c.Server.Port),
	}
	port, err := prompt.Run()
	if err != nil {
		return err
	}
	c.Server.Port, _ = strconv.Atoi(port) // Ignore error, as we've already validated number

	prompt = promptui.Prompt{
		Label:    "Public-facing host",
		Validate: validateDomain,
		Default:  c.App.Host,
	}
	c.App.Host, err = prompt.Run()
	if err != nil {
		return err
	}

	fmt.Println()
	title(" Database setup ")
	fmt.Println()

	prompt = promptui.Prompt{
		Label:    "Username",
		Validate: validateNonEmpty,
		Default:  c.Database.User,
	}
	c.Database.User, err = prompt.Run()
	if err != nil {
		return err
	}

	prompt = promptui.Prompt{
		Label:    "Password",
		Validate: validateNonEmpty,
		Default:  c.Database.Password,
		Mask:     '*',
	}
	c.Database.Password, err = prompt.Run()
	if err != nil {
		return err
	}

	prompt = promptui.Prompt{
		Label:    "Database name",
		Validate: validateNonEmpty,
		Default:  c.Database.Database,
	}
	c.Database.Database, err = prompt.Run()
	if err != nil {
		return err
	}

	prompt = promptui.Prompt{
		Label:    "Host",
		Validate: validateNonEmpty,
		Default:  c.Database.Host,
	}
	c.Database.Host, err = prompt.Run()
	if err != nil {
		return err
	}

	prompt = promptui.Prompt{
		Label:    "Port",
		Validate: validatePort,
		Default:  fmt.Sprintf("%d", c.Database.Port),
	}
	dbPort, err := prompt.Run()
	if err != nil {
		return err
	}
	c.Database.Port, _ = strconv.Atoi(dbPort) // Ignore error, as we've already validated number

	fmt.Println()
	title(" App setup ")
	fmt.Println()

	selPrompt := promptui.Select{
		Label: "Site type",
		Items: []string{"Single user", "Multiple users"},
	}
	_, usersType, err := selPrompt.Run()
	if err != nil {
		return err
	}
	c.App.SingleUser = usersType == "Single user"

	siteNameLabel := "Instance name"
	if c.App.SingleUser {
		siteNameLabel = "Blog name"
	}
	prompt = promptui.Prompt{
		Label:    siteNameLabel,
		Validate: validateNonEmpty,
		Default:  c.App.SiteName,
	}
	c.App.SiteName, err = prompt.Run()
	if err != nil {
		return err
	}

	if !c.App.SingleUser {
		selPrompt = promptui.Select{
			Label: "Registration",
			Items: []string{"Open", "Closed"},
		}
		_, regType, err := selPrompt.Run()
		if err != nil {
			return err
		}
		c.App.OpenRegistration = regType == "Open"
	}

	selPrompt = promptui.Select{
		Label: "Federation",
		Items: []string{"Enabled", "Disabled"},
	}
	_, fedType, err := selPrompt.Run()
	if err != nil {
		return err
	}
	c.App.Federation = fedType == "Enabled"

	if c.App.Federation {
		selPrompt = promptui.Select{
			Label: "Federation usage stats",
			Items: []string{"Public", "Private"},
		}
		_, fedStatsType, err := selPrompt.Run()
		if err != nil {
			return err
		}
		c.App.PublicStats = fedStatsType == "Public"

		selPrompt = promptui.Select{
			Label: "Instance metadata privacy",
			Items: []string{"Public", "Private"},
		}
		_, fedStatsType, err = selPrompt.Run()
		if err != nil {
			return err
		}
		c.App.Private = fedStatsType == "Private"
	}

	return Save(c)
}
