// Copyright 2016 The go-etvchaineum Authors
// This file is part of go-etvchaineum.
//
// go-etvchaineum is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-etvchaineum is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with go-etvchaineum. If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/docker/docker/pkg/reexec"
	"github.com/etvchaineum/go-etvchaineum/internal/cmdtest"
)

func tmpdir(t *testing.T) string {
	dir, err := ioutil.TempDir("", "gech-test")
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

type testgech struct {
	*cmdtest.TestCmd

	// template variables for expect
	Datadir   string
	Etvchainbase string
}

func init() {
	// Run the app if we've been exec'd as "gech-test" in runGech.
	reexec.Register("gech-test", func() {
		if err := app.Run(os.Args); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	})
}

func TestMain(m *testing.M) {
	// check if we have been reexec'd
	if reexec.Init() {
		return
	}
	os.Exit(m.Run())
}

// spawns gech with the given command line args. If the args don't set --datadir, the
// child g gets a temporary data directory.
func runGech(t *testing.T, args ...string) *testgech {
	tt := &testgech{}
	tt.TestCmd = cmdtest.NewTestCmd(t, tt)
	for i, arg := range args {
		switch {
		case arg == "-datadir" || arg == "--datadir":
			if i < len(args)-1 {
				tt.Datadir = args[i+1]
			}
		case arg == "-etvchainbase" || arg == "--etvchainbase":
			if i < len(args)-1 {
				tt.Etvchainbase = args[i+1]
			}
		}
	}
	if tt.Datadir == "" {
		tt.Datadir = tmpdir(t)
		tt.Cleanup = func() { os.RemoveAll(tt.Datadir) }
		args = append([]string{"-datadir", tt.Datadir}, args...)
		// Remove the temporary datadir if someching fails below.
		defer func() {
			if t.Failed() {
				tt.Cleanup()
			}
		}()
	}

	// Boot "gech". This actually runs the test binary but the TestMain
	// function will prevent any tests from running.
	tt.Run("gech-test", args...)

	return tt
}
