package writefreely

import (
	"flag"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/writeas/web-core/log"
	"github.com/writeas/writefreely/config"
)

const (
	staticDir = "static/"
)

type app struct {
	router       *mux.Router
	cfg          *config.Config
	keys         *keychain
	sessionStore *sessions.CookieStore
}

var shttp = http.NewServeMux()

func Serve() {
	createConfig := flag.Bool("create-config", false, "Creates a basic configuration and exits")
	flag.Parse()

	if *createConfig {
		log.Info("Creating configuration...")
		c := config.New()
		log.Info("Saving configuration...")
		config.Save(c)
		os.Exit(0)
	}

	log.Info("Initializing...")

	log.Info("Loading configuration...")
	cfg, err := config.Load()
	if err != nil {
		log.Error("Unable to load configuration: %v", err)
		os.Exit(1)
	}
	app := &app{
		cfg: cfg,
	}

	// Load keys
	log.Info("Loading encryption keys...")
	err = initKeys(app)
	if err != nil {
		log.Error("\n%s\n", err)
	}

	// Initialize modules
	app.sessionStore = initSession(app)

	r := mux.NewRouter()
	handler := NewHandler(app.sessionStore)

	// Handle app routes
	initRoutes(handler, r, app.cfg)

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
	http.ListenAndServe(fmt.Sprintf(":%d", app.cfg.Server.Port), nil)
}

func shutdown(app *app) {
}
