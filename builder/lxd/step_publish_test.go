// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package lxd

import "testing"

func TestParseImageAliasTarget(t *testing.T) {
	csv := "apt-cacher-ng,f60d4ed664b7,CONTAINER,\n" +
		"pihole-old,a3bf25201a8f,CONTAINER,\"old, unused\"\n" +
		"pihole,fdf39469a2a0,CONTAINER,\n"

	cases := map[string]string{
		"pihole":        "fdf39469a2a0",
		"apt-cacher-ng": "f60d4ed664b7",
		"pihole-o":      "",
		"distcc":        "",
	}
	for alias, want := range cases {
		if got := parseImageAliasTarget(csv, alias); got != want {
			t.Errorf("parseImageAliasTarget(%q) = %q, want %q", alias, got, want)
		}
	}
}
