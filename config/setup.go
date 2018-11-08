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

	tmpls := &promptui.PromptTemplates{
		Success: "{{ . | bold | faint }}: ",
	}
	selTmpls := &promptui.SelectTemplates{
		Selected: fmt.Sprintf(`{{.Label}} {{ . | faint }}`, promptui.IconGood),
	}

	prompt := promptui.Prompt{
		Templates: tmpls,
		Label:     "Local port",
		Validate:  validatePort,
		Default:   fmt.Sprintf("%d", c.Server.Port),
	}
	port, err := prompt.Run()
	if err != nil {
		return err
	}
	c.Server.Port, _ = strconv.Atoi(port) // Ignore error, as we've already validated number

	fmt.Println()
	title(" Database setup ")
	fmt.Println()

	prompt = promptui.Prompt{
		Templates: tmpls,
		Label:     "Username",
		Validate:  validateNonEmpty,
		Default:   c.Database.User,
	}
	c.Database.User, err = prompt.Run()
	if err != nil {
		return err
	}

	prompt = promptui.Prompt{
		Templates: tmpls,
		Label:     "Password",
		Validate:  validateNonEmpty,
		Default:   c.Database.Password,
		Mask:      '*',
	}
	c.Database.Password, err = prompt.Run()
	if err != nil {
		return err
	}

	prompt = promptui.Prompt{
		Templates: tmpls,
		Label:     "Database name",
		Validate:  validateNonEmpty,
		Default:   c.Database.Database,
	}
	c.Database.Database, err = prompt.Run()
	if err != nil {
		return err
	}

	prompt = promptui.Prompt{
		Templates: tmpls,
		Label:     "Host",
		Validate:  validateNonEmpty,
		Default:   c.Database.Host,
	}
	c.Database.Host, err = prompt.Run()
	if err != nil {
		return err
	}

	prompt = promptui.Prompt{
		Templates: tmpls,
		Label:     "Port",
		Validate:  validatePort,
		Default:   fmt.Sprintf("%d", c.Database.Port),
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
		Templates: selTmpls,
		Label:     "Site type",
		Items:     []string{"Single user blog", "Multi-user instance"},
	}
	_, usersType, err := selPrompt.Run()
	if err != nil {
		return err
	}
	c.App.SingleUser = usersType == "Single user"
	// TODO: if c.App.SingleUser {
	//   prompt for username
	//   prompt for password
	//   create blog

	siteNameLabel := "Instance name"
	if c.App.SingleUser {
		siteNameLabel = "Blog name"
	}
	prompt = promptui.Prompt{
		Templates: tmpls,
		Label:     siteNameLabel,
		Validate:  validateNonEmpty,
		Default:   c.App.SiteName,
	}
	c.App.SiteName, err = prompt.Run()
	if err != nil {
		return err
	}

	prompt = promptui.Prompt{
		Templates: tmpls,
		Label:     "Public URL",
		Validate:  validateDomain,
		Default:   c.App.Host,
	}
	c.App.Host, err = prompt.Run()
	if err != nil {
		return err
	}

	if !c.App.SingleUser {
		selPrompt = promptui.Select{
			Templates: selTmpls,
			Label:     "Registration",
			Items:     []string{"Open", "Closed"},
		}
		_, regType, err := selPrompt.Run()
		if err != nil {
			return err
		}
		c.App.OpenRegistration = regType == "Open"

		prompt = promptui.Prompt{
			Templates: tmpls,
			Label:     "Max blogs per user",
			Default:   fmt.Sprintf("%d", c.App.MaxBlogs),
		}
		maxBlogs, err := prompt.Run()
		if err != nil {
			return err
		}
		c.App.MaxBlogs, _ = strconv.Atoi(maxBlogs) // Ignore error, as we've already validated number
	}

	selPrompt = promptui.Select{
		Templates: selTmpls,
		Label:     "Federation",
		Items:     []string{"Enabled", "Disabled"},
	}
	_, fedType, err := selPrompt.Run()
	if err != nil {
		return err
	}
	c.App.Federation = fedType == "Enabled"

	if c.App.Federation {
		selPrompt = promptui.Select{
			Templates: selTmpls,
			Label:     "Federation usage stats",
			Items:     []string{"Public", "Private"},
		}
		_, fedStatsType, err := selPrompt.Run()
		if err != nil {
			return err
		}
		c.App.PublicStats = fedStatsType == "Public"

		selPrompt = promptui.Select{
			Templates: selTmpls,
			Label:     "Instance metadata privacy",
			Items:     []string{"Public", "Private"},
		}
		_, fedStatsType, err = selPrompt.Run()
		if err != nil {
			return err
		}
		c.App.Private = fedStatsType == "Private"
	}

	return Save(c)
}
