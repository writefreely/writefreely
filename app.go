package writefreely

import (
	"database/sql"
	"flag"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"html/template"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"github.com/gorilla/sessions"
	"github.com/manifoldco/promptui"
	"github.com/writeas/web-core/converter"
	"github.com/writeas/web-core/log"
	"github.com/writeas/writefreely/config"
	"github.com/writeas/writefreely/page"
)

const (
	staticDir       = "static/"
	assumedTitleLen = 80
	postsPerPage    = 10

	serverSoftware = "WriteFreely"
	softwareURL    = "https://writefreely.org"

	softwareVer = "0.3"
)

var (
	debugging bool

	// DEPRECATED VARS
	// TODO: pass app.cfg into GetCollection* calls so we can get these values
	// from Collection methods and we no longer need these.
	hostName     string
	isSingleUser bool
)

type app struct {
	router       *mux.Router
	db           *datastore
	cfg          *config.Config
	keys         *keychain
	sessionStore *sessions.CookieStore
	formDecoder  *schema.Decoder
}

// handleViewHome shows page at root path. Will be the Pad if logged in and the
// catch-all landing page otherwise.
func handleViewHome(app *app, w http.ResponseWriter, r *http.Request) error {
	if app.cfg.App.SingleUser {
		// Render blog index
		return handleViewCollection(app, w, r)
	}

	// Multi-user instance
	u := getUserSession(app, r)
	if u != nil {
		// User is logged in, so show the Pad
		return handleViewPad(app, w, r)
	}

	p := struct {
		page.StaticPage
		Flashes []template.HTML
	}{
		StaticPage: pageForReq(app, r),
	}

	// Get error messages
	session, err := app.sessionStore.Get(r, cookieName)
	if err != nil {
		// Ignore this
		log.Error("Unable to get session in handleViewHome; ignoring: %v", err)
	}
	flashes, _ := getSessionFlashes(app, w, r, session)
	for _, flash := range flashes {
		p.Flashes = append(p.Flashes, template.HTML(flash))
	}

	// Show landing page
	return renderPage(w, "landing.tmpl", p)
}

func handleTemplatedPage(app *app, w http.ResponseWriter, r *http.Request, t *template.Template) error {
	p := struct {
		page.StaticPage
		Content template.HTML
		Updated string
	}{
		StaticPage: pageForReq(app, r),
	}
	if r.URL.Path == "/about" || r.URL.Path == "/privacy" {
		var c string
		var updated *time.Time
		var err error

		if r.URL.Path == "/about" {
			c, err = getAboutPage(app)
		} else {
			c, updated, err = getPrivacyPage(app)
		}

		if err != nil {
			return err
		}
		p.Content = template.HTML(applyMarkdown([]byte(c)))
		if updated != nil {
			p.Updated = updated.Format("January 2, 2006")
		}
	}

	// Serve templated page
	err := t.ExecuteTemplate(w, "base", p)
	if err != nil {
		log.Error("Unable to render page: %v", err)
	}
	return nil
}

func pageForReq(app *app, r *http.Request) page.StaticPage {
	p := page.StaticPage{
		AppCfg:  app.cfg.App,
		Path:    r.URL.Path,
		Version: "v" + softwareVer,
	}

	// Add user information, if given
	var u *User
	accessToken := r.FormValue("t")
	if accessToken != "" {
		userID := app.db.GetUserID(accessToken)
		if userID != -1 {
			var err error
			u, err = app.db.GetUserByID(userID)
			if err == nil {
				p.Username = u.Username
			}
		}
	} else {
		u = getUserSession(app, r)
		if u != nil {
			p.Username = u.Username
		}
	}

	return p
}

var shttp = http.NewServeMux()
var fileRegex = regexp.MustCompile("/([^/]*\\.[^/]*)$")

func Serve() {
	debugPtr := flag.Bool("debug", false, "Enables debug logging.")
	createConfig := flag.Bool("create-config", false, "Creates a basic configuration and exits")
	doConfig := flag.Bool("config", false, "Run the configuration process")
	genKeys := flag.Bool("gen-keys", false, "Generate encryption and authentication keys")
	createSchema := flag.Bool("init-db", false, "Initialize app database")
	resetPassUser := flag.String("reset-pass", "", "Reset the given user's password")
	outputVersion := flag.Bool("v", false, "Output the current version")
	flag.Parse()

	debugging = *debugPtr

	app := &app{}

	if *outputVersion {
		fmt.Println(serverSoftware + " " + softwareVer)
		os.Exit(0)
	} else if *createConfig {
		log.Info("Creating configuration...")
		c := config.New()
		log.Info("Saving configuration...")
		err := config.Save(c)
		if err != nil {
			log.Error("Unable to save configuration: %v", err)
			os.Exit(1)
		}
		os.Exit(0)
	} else if *doConfig {
		d, err := config.Configure()
		if err != nil {
			log.Error("Unable to configure: %v", err)
			os.Exit(1)
		}
		if d.User != nil {
			app.cfg = d.Config
			connectToDatabase(app)
			defer shutdown(app)

			u := &User{
				Username:   d.User.Username,
				HashedPass: d.User.HashedPass,
				Created:    time.Now().Truncate(time.Second).UTC(),
			}

			// Create blog
			log.Info("Creating user %s...\n", u.Username)
			err = app.db.CreateUser(u, app.cfg.App.SiteName)
			if err != nil {
				log.Error("Unable to create user: %s", err)
				os.Exit(1)
			}
			log.Info("Done!")
		}
		os.Exit(0)
	} else if *genKeys {
		errStatus := 0

		err := generateKey(emailKeyPath)
		if err != nil {
			errStatus = 1
		}
		err = generateKey(cookieAuthKeyPath)
		if err != nil {
			errStatus = 1
		}
		err = generateKey(cookieKeyPath)
		if err != nil {
			errStatus = 1
		}

		os.Exit(errStatus)
	} else if *createSchema {
		log.Info("Loading configuration...")
		cfg, err := config.Load()
		if err != nil {
			log.Error("Unable to load configuration: %v", err)
			os.Exit(1)
		}
		app.cfg = cfg
		connectToDatabase(app)
		defer shutdown(app)

		schema, err := ioutil.ReadFile("schema.sql")
		if err != nil {
			log.Error("Unable to load schema.sql: %v", err)
			os.Exit(1)
		}

		tblReg := regexp.MustCompile("CREATE TABLE (IF NOT EXISTS )?`([a-z_]+)`")

		queries := strings.Split(string(schema), ";\n")
		for _, q := range queries {
			if strings.TrimSpace(q) == "" {
				continue
			}
			parts := tblReg.FindStringSubmatch(q)
			if len(parts) >= 3 {
				log.Info("Creating table %s...", parts[2])
			} else {
				log.Info("Creating table ??? (Weird query) No match in: %v", parts)
			}
			_, err = app.db.Exec(q)
			if err != nil {
				log.Error("%s", err)
			} else {
				log.Info("Created.")
			}
		}
		os.Exit(0)
	} else if *resetPassUser != "" {
		// Connect to the database
		log.Info("Loading configuration...")
		cfg, err := config.Load()
		if err != nil {
			log.Error("Unable to load configuration: %v", err)
			os.Exit(1)
		}
		app.cfg = cfg
		connectToDatabase(app)
		defer shutdown(app)

		// Fetch user
		u, err := app.db.GetUserForAuth(*resetPassUser)
		if err != nil {
			log.Error("Get user: %s", err)
			os.Exit(1)
		}

		// Prompt for new password
		prompt := promptui.Prompt{
			Templates: &promptui.PromptTemplates{
				Success: "{{ . | bold | faint }}: ",
			},
			Label: "New password",
			Mask:  '*',
		}
		newPass, err := prompt.Run()
		if err != nil {
			log.Error("%s", err)
			os.Exit(1)
		}

		// Do the update
		log.Info("Updating...")
		err = adminResetPassword(app, u, newPass)
		if err != nil {
			log.Error("%s", err)
			os.Exit(1)
		}
		log.Info("Success.")
		os.Exit(0)
	}

	log.Info("Initializing...")

	log.Info("Loading configuration...")
	cfg, err := config.Load()
	if err != nil {
		log.Error("Unable to load configuration: %v", err)
		os.Exit(1)
	}
	app.cfg = cfg

	hostName = cfg.App.Host
	isSingleUser = cfg.App.SingleUser
	app.cfg.Server.Dev = *debugPtr

	initTemplates()

	// Load keys
	log.Info("Loading encryption keys...")
	err = initKeys(app)
	if err != nil {
		log.Error("\n%s\n", err)
	}

	// Initialize modules
	app.sessionStore = initSession(app)
	app.formDecoder = schema.NewDecoder()
	app.formDecoder.RegisterConverter(converter.NullJSONString{}, converter.ConvertJSONNullString)
	app.formDecoder.RegisterConverter(converter.NullJSONBool{}, converter.ConvertJSONNullBool)
	app.formDecoder.RegisterConverter(sql.NullString{}, converter.ConvertSQLNullString)
	app.formDecoder.RegisterConverter(sql.NullBool{}, converter.ConvertSQLNullBool)
	app.formDecoder.RegisterConverter(sql.NullInt64{}, converter.ConvertSQLNullInt64)
	app.formDecoder.RegisterConverter(sql.NullFloat64{}, converter.ConvertSQLNullFloat64)

	// Check database configuration
	if app.cfg.Database.User == "" || app.cfg.Database.Password == "" {
		log.Error("Database user or password not set.")
		os.Exit(1)
	}
	if app.cfg.Database.Host == "" {
		app.cfg.Database.Host = "localhost"
	}
	if app.cfg.Database.Database == "" {
		app.cfg.Database.Database = "writeas"
	}

	connectToDatabase(app)
	defer shutdown(app)

	r := mux.NewRouter()
	handler := NewHandler(app)
	handler.SetErrorPages(&ErrorPages{
		NotFound:            pages["404-general.tmpl"],
		Gone:                pages["410.tmpl"],
		InternalServerError: pages["500.tmpl"],
		Blank:               pages["blank.tmpl"],
	})

	// Handle app routes
	initRoutes(handler, r, app.cfg, app.db)

	// Handle static files
	fs := http.FileServer(http.Dir(staticDir))
	shttp.Handle("/", fs)
	r.PathPrefix("/").Handler(fs)

	// Handle shutdown
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		log.Info("Shutting down...")
		shutdown(app)
		log.Info("Done.")
		os.Exit(0)
	}()

	// Start web application server
	http.Handle("/", r)
	log.Info("Serving on http://localhost:%d\n", app.cfg.Server.Port)
	log.Info("---")
	err = http.ListenAndServe(fmt.Sprintf(":%d", app.cfg.Server.Port), nil)
	if err != nil {
		log.Error("Unable to start: %v", err)
		os.Exit(1)
	}
}

func connectToDatabase(app *app) {
	log.Info("Connecting to database...")
	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=%s", app.cfg.Database.User, app.cfg.Database.Password, app.cfg.Database.Host, app.cfg.Database.Port, app.cfg.Database.Database, url.QueryEscape(time.Local.String())))
	if err != nil {
		log.Error("%s", err)
		os.Exit(1)
	}
	app.db = &datastore{db}
	app.db.SetMaxOpenConns(50)
}

func shutdown(app *app) {
	log.Info("Closing database connection...")
	app.db.Close()
}
