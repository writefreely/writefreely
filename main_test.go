package writefreely

import (
	"context"
	"database/sql"
	"encoding/gob"
	"errors"
	"fmt"
	uuid "github.com/nu7hatch/gouuid"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"
)

var testDB *sql.DB

type ScopedTestBody func(*sql.DB)

// TestMain provides testing infrastructure within this package.
func TestMain(m *testing.M) {
	rand.Seed(time.Now().UTC().UnixNano())
	gob.Register(&User{})

	if runMySQLTests() {
		var err error

		testDB, err = initMySQL(os.Getenv("WF_USER"), os.Getenv("WF_PASSWORD"), os.Getenv("WF_DB"), os.Getenv("WF_HOST"))
		if err != nil {
			fmt.Println(err)
			return
		}
	}

	code := m.Run()
	if runMySQLTests() {
		if closeErr := testDB.Close(); closeErr != nil {
			fmt.Println(closeErr)
		}
	}
	os.Exit(code)
}

func runMySQLTests() bool {
	return len(os.Getenv("TEST_MYSQL")) > 0
}

func initMySQL(dbUser, dbPassword, dbName, dbHost string) (*sql.DB, error) {
	if dbUser == "" || dbPassword == "" {
		return nil, errors.New("database user or password not set")
	}
	if dbHost == "" {
		dbHost = "localhost"
	}
	if dbName == "" {
		dbName = "writefreely"
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:3306)/%s?charset=utf8mb4&parseTime=true", dbUser, dbPassword, dbHost, dbName)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	if err := ensureMySQL(db); err != nil {
		return nil, err
	}
	return db, nil
}

func ensureMySQL(db *sql.DB) error {
	if err := db.Ping(); err != nil {
		return err
	}
	db.SetMaxOpenConns(250)
	return nil
}

// withTestDB provides a scoped database connection.
func withTestDB(t *testing.T, testBody ScopedTestBody) {
	db, cleanup, err := newTestDatabase(testDB,
		os.Getenv("WF_USER"),
		os.Getenv("WF_PASSWORD"),
		os.Getenv("WF_DB"),
		os.Getenv("WF_HOST"),
	)
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, cleanup())
	}()

	testBody(db)
}

// newTestDatabase creates a new temporary test database. When a test
// database connection is returned, it will have created a new database and
// initialized it with tables from a reference database.
func newTestDatabase(base *sql.DB, dbUser, dbPassword, dbName, dbHost string) (*sql.DB, func() error, error) {
	var err error
	var baseName = dbName

	if baseName == "" {
		row := base.QueryRow("SELECT DATABASE()")
		err := row.Scan(&baseName)
		if err != nil {
			return nil, nil, err
		}
	}
	tUUID, _ := uuid.NewV4()
	suffix := strings.Replace(tUUID.String(), "-", "_", -1)
	newDBName := baseName + suffix
	_, err = base.Exec("CREATE DATABASE " + newDBName)
	if err != nil {
		return nil, nil, err
	}
	newDB, err := initMySQL(dbUser, dbPassword, newDBName, dbHost)
	if err != nil {
		return nil, nil, err
	}

	rows, err := base.Query("SHOW TABLES IN " + baseName)
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, nil, err
		}
		query := fmt.Sprintf("CREATE TABLE %s LIKE %s.%s", tableName, baseName, tableName)
		if _, err := newDB.Exec(query); err != nil {
			return nil, nil, err
		}
	}

	cleanup := func() error {
		if closeErr := newDB.Close(); closeErr != nil {
			fmt.Println(closeErr)
		}

		_, err = base.Exec("DROP DATABASE " + newDBName)
		return err
	}
	return newDB, cleanup, nil
}

func countRows(t *testing.T, ctx context.Context, db *sql.DB, count int, query string, args ...interface{}) {
	var returned int
	err := db.QueryRowContext(ctx, query, args...).Scan(&returned)
	assert.NoError(t, err, "error executing query %s and args %s", query, args)
	assert.Equal(t, count, returned, "unexpected return count %d, expected %d from %s and args %s", returned, count, query, args)
}