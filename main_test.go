package writefreely

import (
	"encoding/gob"
	"math/rand"
	"os"
	"testing"
	"time"
)

// TestMain provides testing infrastructure within this package.
func TestMain(m *testing.M) {
	rand.Seed(time.Now().UTC().UnixNano())

	gob.Register(&User{})

	os.Exit(m.Run())
}