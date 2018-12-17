package writefreely

import (
	"database/sql"
	"flag"
	"fmt"
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

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"

	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"github.com/gorilla/sessions"
	"github.com/manifoldco/promptui"
	"github.com/writeas/go-strip-markdown"
	"github.com/writeas/web-core/auth"
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
)

var (
	debugging bool

	// Software version can be set from git env using -ldflags
	softwareVer = "0.6.0"

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
	cfgFile      string
	keys         *keychain
	sessionStore *sessions.CookieStore
	formDecoder  *schema.Decoder

	timeline *localTimeline
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
		Content      template.HTML
		PlainContent string
		Updated      string

		AboutStats *InstanceStats
	}{
		StaticPage: pageForReq(app, r),
	}
	if r.URL.Path == "/about" || r.URL.Path == "/privacy" {
		var c string
		var updated *time.Time
		var err error

		if r.URL.Path == "/about" {
			c, err = getAboutPage(app)

			// Fetch stats
			p.AboutStats = &InstanceStats{}
			p.AboutStats.NumPosts, _ = app.db.GetTotalPosts()
			p.AboutStats.NumBlogs, _ = app.db.GetTotalCollections()
		} else {
			c, updated, err = getPrivacyPage(app)
		}

		if err != nil {
			return err
		}
		p.Content = template.HTML(applyMarkdown([]byte(c)))
		p.PlainContent = shortPostDescription(stripmd.Strip(c))
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
	createAdmin := flag.String("create-admin", "", "Create an admin with the given username:password")
	resetPassUser := flag.String("reset-pass", "", "Reset the given user's password")
	configFile := flag.String("c", "config.ini", "The configuration file to use")
	outputVersion := flag.Bool("v", false, "Output the current version")
	flag.Parse()

	debugging = *debugPtr

	app := &app{
		cfgFile: *configFile,
	}

	if *outputVersion {
		fmt.Println(serverSoftware + " " + softwareVer)
		os.Exit(0)
	} else if *createConfig {
		log.Info("Creating configuration...")
		c := config.New()
		log.Info("Saving configuration %s...", app.cfgFile)
		err := config.Save(c, app.cfgFile)
		if err != nil {
			log.Error("Unable to save configuration: %v", err)
			os.Exit(1)
		}
		os.Exit(0)
	} else if *doConfig {
		d, err := config.Configure(app.cfgFile)
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
		loadConfig(app)
		connectToDatabase(app)
		defer shutdown(app)

		schemaFileName := "schema.sql"

		if app.cfg.Database.Type == "sqlite3" {
			schemaFileName = "sqlite.sql"
		}

		schema, err := ioutil.ReadFile(schemaFileName)
		if err != nil {
			log.Error("Unable to load schema file: %v", err)
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
	} else if *createAdmin != "" {
		adminCreateUser(app, *createAdmin, true)
	} else if *resetPassUser != "" {
		// Connect to the database
		loadConfig(app)
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

	loadConfig(app)

	hostName = app.cfg.App.Host
	isSingleUser = app.cfg.App.SingleUser
	app.cfg.Server.Dev = *debugPtr

	initTemplates()

	// Load keys
	log.Info("Loading encryption keys...")
	err := initKeys(app)
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
		app.cfg.Database.Database = "writefreely"
	}

	connectToDatabase(app)
	defer shutdown(app)

	// Test database connection
	err = app.db.Ping()
	if err != nil {
		log.Error("Database ping failed: %s", err)
	}

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

	// Handle local timeline, if enabled
	if app.cfg.App.LocalTimeline {
		log.Info("Initializing local timeline...")
		initLocalTimeline(app)
	}

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

	http.Handle("/", r)

	// Start web application server
	var bindAddress = app.cfg.Server.Bind
	if bindAddress == "" {
		bindAddress = "localhost"
	}
	if app.cfg.IsSecureStandalone() {
		log.Info("Serving redirects on http://%s:80", bindAddress)
		go func() {
			err = http.ListenAndServe(
				fmt.Sprintf("%s:80", bindAddress), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					http.Redirect(w, r, app.cfg.App.Host, http.StatusMovedPermanently)
				}))
			log.Error("Unable to start redirect server: %v", err)
		}()

		log.Info("Serving on https://%s:443", bindAddress)
		log.Info("---")
		err = http.ListenAndServeTLS(
			fmt.Sprintf("%s:443", bindAddress), app.cfg.Server.TLSCertPath, app.cfg.Server.TLSKeyPath, nil)
	} else {
		log.Info("Serving on http://%s:%d\n", bindAddress, app.cfg.Server.Port)
		log.Info("---")
		err = http.ListenAndServe(fmt.Sprintf("%s:%d", bindAddress, app.cfg.Server.Port), nil)
	}
	if err != nil {
		log.Error("Unable to start: %v", err)
		os.Exit(1)
	}
}

func loadConfig(app *app) {
	log.Info("Loading %s configuration...", app.cfgFile)
	cfg, err := config.Load(app.cfgFile)
	if err != nil {
		log.Error("Unable to load configuration: %v", err)
		os.Exit(1)
	}
	app.cfg = cfg
}

func connectToDatabase(app *app) {
	log.Info("Connecting to %s database...", app.cfg.Database.Type)

	var db *sql.DB
	var err error
	if app.cfg.Database.Type == "mysql" {
		db, err = sql.Open(app.cfg.Database.Type, fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=%s", app.cfg.Database.User, app.cfg.Database.Password, app.cfg.Database.Host, app.cfg.Database.Port, app.cfg.Database.Database, url.QueryEscape(time.Local.String())))
		db.SetMaxOpenConns(50)
	} else if app.cfg.Database.Type == "sqlite3" {
		if app.cfg.Database.FileName == "" {
			log.Error("SQLite database filename value in config.ini is empty.")
			os.Exit(1)
		}
		db, err = sql.Open("sqlite3", app.cfg.Database.FileName+"?parseTime=true&cached=shared")
		db.SetMaxOpenConns(1)
	} else {
		log.Error("Invalid database type '%s'. Only 'mysql' and 'sqlite3' are supported right now.", app.cfg.Database.Type)
		os.Exit(1)
	}
	if err != nil {
		log.Error("%s", err)
		os.Exit(1)
	}
	app.db = &datastore{db, app.cfg.Database.Type}
}

func shutdown(app *app) {
	log.Info("Closing database connection...")
	app.db.Close()
}

func adminCreateUser(app *app, credStr string, isAdmin bool) {
	// Create an admin user with --create-admin
	creds := strings.Split(credStr, ":")
	if len(creds) != 2 {
		log.Error("usage: writefreely --create-admin username:password")
		os.Exit(1)
	}

	loadConfig(app)
	connectToDatabase(app)
	defer shutdown(app)

	// Ensure an admin / first user doesn't already exist
	if u, _ := app.db.GetUserByID(1); u != nil {
		log.Error("Admin user already exists (%s). Aborting.", u.Username)
		os.Exit(1)
	}

	// Create the user
	username := creds[0]
	password := creds[1]

	hashedPass, err := auth.HashPass([]byte(password))
	if err != nil {
		log.Error("Unable to hash password: %v", err)
		os.Exit(1)
	}

	u := &User{
		Username:   username,
		HashedPass: hashedPass,
		Created:    time.Now().Truncate(time.Second).UTC(),
	}

	log.Info("Creating user %s...", u.Username)
	err = app.db.CreateUser(u, "")
	if err != nil {
		log.Error("Unable to create user: %s", err)
		os.Exit(1)
	}
	log.Info("Done!")
	os.Exit(0)
}
