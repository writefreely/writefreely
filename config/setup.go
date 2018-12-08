package config

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/manifoldco/promptui"
	"github.com/mitchellh/go-wordwrap"
	"github.com/writeas/web-core/auth"
	"strconv"
)

type SetupData struct {
	User   *UserCreation
	Config *Config
}

func Configure() (*SetupData, error) {
	data := &SetupData{}
	var err error

	data.Config, err = Load()
	var action string
	isNewCfg := false
	if err != nil {
		fmt.Println("No configuration yet. Creating new.")
		data.Config = New()
		action = "generate"
		isNewCfg = true
	} else {
		fmt.Println("Configuration loaded.")
		action = "update"
	}
	title := color.New(color.Bold, color.BgGreen).PrintFunc()

	intro := color.New(color.Bold, color.FgWhite).PrintlnFunc()
	fmt.Println()
	intro("  ✍ Write Freely Configuration ✍")
	fmt.Println()
	fmt.Println(wordwrap.WrapString("  This quick configuration process will "+action+" the application's config file, "+FileName+".\n\n  It validates your input along the way, so you can be sure any future errors aren't caused by a bad configuration. If you'd rather configure your server manually, instead run: writefreely --create-config and edit that file.", 75))
	fmt.Println()

	title(" Server setup ")
	fmt.Println()

	tmpls := &promptui.PromptTemplates{
		Success: "{{ . | bold | faint }}: ",
	}
	selTmpls := &promptui.SelectTemplates{
		Selected: fmt.Sprintf(`{{.Label}} {{ . | faint }}`),
	}

	// Environment selection
	selPrompt := promptui.Select{
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

	var prompt promptui.Prompt
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
			Items:     []string{"Insecure (port 80)", "Secure (port 443)"},
		}
		sel, _, err := selPrompt.Run()
		if err != nil {
			return data, err
		}
		if sel == 0 {
			data.Config.Server.Port = 80
			data.Config.Server.TLSCertPath = ""
			data.Config.Server.TLSKeyPath = ""
		} else if sel == 1 {
			data.Config.Server.Port = 443

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
		}
	} else {
		data.Config.Server.TLSCertPath = ""
		data.Config.Server.TLSKeyPath = ""
	}

	fmt.Println()
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
			Label:     "Federation usage stats",
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

	return data, Save(data.Config)
}
