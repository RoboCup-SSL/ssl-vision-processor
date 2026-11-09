package main

import (
	"fmt"
	"os"
	"slices"
	"sync"
	"testing"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/api"
)

// TestMain fails the run if any route answered an error code its api.Doc
// doesn't declare, across every test here: the spec's error list per route
// must be complete.
func TestMain(m *testing.M) {
	var (
		mu         sync.Mutex
		undeclared []string
	)

	api.CheckDeclared = func(pattern, code string) {
		mu.Lock()
		defer mu.Unlock()

		if entry := pattern + ": " + code; !slices.Contains(undeclared, entry) {
			undeclared = append(undeclared, entry)
		}
	}

	code := m.Run()

	if len(undeclared) > 0 {
		fmt.Fprintln(os.Stderr, "routes answered error codes their api.Doc doesn't declare:")

		for _, entry := range undeclared {
			fmt.Fprintln(os.Stderr, "  "+entry)
		}

		code = 1
	}

	os.Exit(code)
}
