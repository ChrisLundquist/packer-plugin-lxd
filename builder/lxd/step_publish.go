// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package lxd

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

type stepPublish struct{}

func (s *stepPublish) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {

	var remote string = ""

	config := state.Get("config").(*Config)
	ui := state.Get("ui").(packersdk.Ui)

	if config.SkipPublish {
		ui.Say("skip_publish is true. Skipping step for publish")
		return multistep.ActionContinue
	}

	if config.PublishRemoteName != "" {
		remote = config.PublishRemoteName + ":"
	}
	alias := remote + config.OutputImage

	// LXD won't publish onto an alias that already exists. Under -force we
	// remove the old alias first. The previous image stays (unaliased) so it
	// can still be rolled back to, and gets its alias back if publishing
	// fails. `lxc publish --reuse` would delete it instead, and only exists
	// since LXD 5.0.3/5.14.
	oldImage, err := imageAliasTarget(remote, config.OutputImage)
	if err != nil {
		err := fmt.Errorf("Error checking for existing image alias %s: %s", alias, err)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	if oldImage != "" && !config.PackerForce {
		err := fmt.Errorf("Image alias %s already exists; run with -force to replace it", alias)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	name := config.ContainerName
	stop_args := []string{
		// We created the container with "--ephemeral=false" so we know it is safe to stop.
		"stop", name,
	}

	ui.Say("Stopping container...")
	_, err = LXDCommand(stop_args...)
	if err != nil {
		err := fmt.Errorf("Error stopping container: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	if oldImage != "" {
		ui.Say(fmt.Sprintf("Removing image alias %s from image %s...", alias, oldImage))
		if _, err := LXDCommand("image", "alias", "delete", alias); err != nil {
			err := fmt.Errorf("Error removing existing image alias: %s", err)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}
	}

	publish_args := []string{
		"publish", name, remote, "--alias", config.OutputImage,
	}

	for k, v := range config.PublishProperties {
		publish_args = append(publish_args, fmt.Sprintf("%s=%s", k, v))
	}

	ui.Say("Publishing container...")
	stdoutString, err := LXDCommand(publish_args...)
	if err != nil {
		if oldImage != "" {
			if _, aliasErr := LXDCommand("image", "alias", "create", alias, oldImage); aliasErr != nil {
				ui.Error(fmt.Sprintf("Error restoring image alias %s to %s: %s", alias, oldImage, aliasErr))
			}
		}
		err := fmt.Errorf("Error publishing container: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	r := regexp.MustCompile("([0-9a-fA-F]+)$")
	fingerprint := r.FindAllStringSubmatch(stdoutString, -1)[0][0]

	ui.Say(fmt.Sprintf("Created image: %s", fingerprint))

	state.Put("imageFingerprint", fingerprint)

	return multistep.ActionContinue
}

func (s *stepPublish) Cleanup(state multistep.StateBag) {}

// imageAliasTarget returns the (short) fingerprint of the image that alias
// points to on remote ("" for the default remote, or "name:"), or "" if there
// is no such alias.
func imageAliasTarget(remote string, alias string) (string, error) {
	args := []string{"image", "alias", "list"}
	if remote != "" {
		args = append(args, remote)
	}
	args = append(args, "--format", "csv")

	stdout, err := LXDCommand(args...)
	if err != nil {
		return "", err
	}

	return parseImageAliasTarget(stdout, alias), nil
}

// parseImageAliasTarget finds alias in `lxc image alias list --format csv`
// output (ALIAS,FINGERPRINT,...).
func parseImageAliasTarget(csv string, alias string) string {
	for _, line := range strings.Split(csv, "\n") {
		fields := strings.Split(line, ",")
		if len(fields) >= 2 && fields[0] == alias {
			return fields[1]
		}
	}

	return ""
}
