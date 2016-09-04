// log holds loggers for the application
package log

import (
	"log"
	"os"
)

var (
	i *log.Logger
	e *log.Logger
)

// Init creates the local loggers used in the app.
func Init() {
	i = log.New(os.Stdout, "", log.Ldate|log.Ltime)
	e = log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
}

// Info logs an informational message to Stdout.
func Info(s string, v ...interface{}) {
	i.Printf(s, v)
}

// Error logs an error to Stderr.
func Error(s string, v ...interface{}) {
	e.Printf(s, v)
}
