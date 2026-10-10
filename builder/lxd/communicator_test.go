// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package lxd

import (
	"os"
	"path/filepath"
	"testing"

	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

func TestCommunicator_ImplementsCommunicator(t *testing.T) {
	var raw interface{}
	raw = &Communicator{}
	if _, ok := raw.(packersdk.Communicator); !ok {
		t.Fatalf("Communicator should be a communicator")
	}
}

func TestFilePushCommand(t *testing.T) {
	path := filepath.Join(t.TempDir(), "unbound.conf")
	if err := os.WriteFile(path, nil, 0644); err != nil {
		t.Fatal(err)
	}
	// WriteFile's mode is subject to the umask.
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name    string
		fi      *os.FileInfo
		newFile bool
		want    string
	}{
		{"new file", &fi, true, "lxc file push --uid=0 --gid=0 --mode=0644 - c1/etc/unbound.conf"},
		{"new file without file info", nil, true, "lxc file push --uid=0 --gid=0 - c1/etc/unbound.conf"},
		{"existing file", &fi, false, "lxc file push - c1/etc/unbound.conf"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := filePushCommand("c1/etc/unbound.conf", tc.fi, tc.newFile)
			if got != tc.want {
				t.Errorf("filePushCommand() = %q, want %q", got, tc.want)
			}
		})
	}
}

// Acceptance tests
// TODO Execute a command
// TODO Upload a file
// TODO Download a file
// TODO Upload a Directory
