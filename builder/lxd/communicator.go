// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package lxd

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

type Communicator struct {
	ContainerName string
	CmdWrapper    CommandWrapper
}

func (c *Communicator) Start(ctx context.Context, cmd *packersdk.RemoteCmd) error {
	localCmd, err := c.Execute(cmd.Command)

	if err != nil {
		return err
	}

	localCmd.Stdin = cmd.Stdin
	localCmd.Stdout = cmd.Stdout
	localCmd.Stderr = cmd.Stderr
	if err := localCmd.Start(); err != nil {
		return err
	}

	go func() {
		exitStatus := 0
		if err := localCmd.Wait(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitStatus = 1

				// There is no process-independent way to get the REAL
				// exit status so we just try to go deeper.
				if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
					exitStatus = status.ExitStatus()
				}
			}
		}

		log.Printf(
			"lxc exec execution exited with '%d': '%s'",
			exitStatus, cmd.Command)
		cmd.SetExited(exitStatus)
	}()

	return nil
}

func (c *Communicator) Upload(dst string, r io.Reader, fi *os.FileInfo) error {
	ctx := context.TODO()

	// find out if the place we are pushing to is a directory
	isDir, err := c.remoteTest(ctx, "-d", dst)
	if err != nil {
		log.Printf("Unable to check whether remote path is a dir: %s", err)
		return err
	}

	if isDir {
		if fi == nil || *fi == nil {
			return fmt.Errorf("cannot upload into directory %s without a source file name", dst)
		}
		log.Printf("path is a directory; copying file into directory.")
		dst = filepath.Join(dst, (*fi).Name())
	}

	exists, err := c.remoteTest(ctx, "-e", dst)
	if err != nil {
		log.Printf("Unable to check whether remote path exists: %s", err)
		return err
	}

	fileDestination := filepath.Join(c.ContainerName, dst)
	cpCmd, err := c.CmdWrapper(filePushCommand(fileDestination, fi, !exists))
	if err != nil {
		return err
	}

	log.Printf("Running copy command: %s", cpCmd)
	command := ShellCommand(cpCmd)
	command.Stdin = r

	return command.Run()
}

// remoteTest reports whether `test <flag> <path>` succeeds in the container.
func (c *Communicator) remoteTest(ctx context.Context, flag string, path string) (bool, error) {
	cmd := &packersdk.RemoteCmd{Command: fmt.Sprintf(`test %s "%s"`, flag, path)}
	if err := c.Start(ctx, cmd); err != nil {
		return false, err
	}
	cmd.Wait()

	return cmd.ExitStatus() == 0, nil
}

// filePushCommand builds the `lxc file push` that streams a file from stdin to
// dst (<container>/<path>). Overwriting an existing file keeps its owner and
// mode. A new file is created root-owned with the source file's permissions:
// without the flags lxc takes both from its stdin pipe, i.e. the host user's
// uid (ubuntu inside the container) and mode 0600, which root-owned services
// can't read.
func filePushCommand(dst string, fi *os.FileInfo, newFile bool) string {
	if !newFile {
		return fmt.Sprintf("lxc file push - %s", dst)
	}

	mode := ""
	if fi != nil && *fi != nil {
		mode = fmt.Sprintf(" --mode=%04o", (*fi).Mode().Perm())
	}

	return fmt.Sprintf("lxc file push --uid=0 --gid=0%s - %s", mode, dst)
}

func (c *Communicator) UploadDir(dst string, src string, exclude []string) error {
	fileDestination := fmt.Sprintf("%s/%s", c.ContainerName, dst)

	pushCommand := ""

	if strings.HasSuffix(src, "/") {
		pushCommand = fmt.Sprintf("lxc file push --debug -pr %s* %s", src, fileDestination)
	} else {
		pushCommand = fmt.Sprintf("lxc file push --debug -pr %s %s", src, fileDestination)
	}

	log.Printf(pushCommand)
	cp, err := c.CmdWrapper(pushCommand)
	if err != nil {
		log.Printf("Error running cp command: %s", err)
		return err
	}

	cpCmd := ShellCommand(cp)

	log.Printf("Running cp command: %s", cp)
	err = cpCmd.Run()
	if err != nil {
		log.Printf("Error running cp command: %s", err)
		return err
	}

	return nil
}

func (c *Communicator) Download(src string, w io.Writer) error {
	cpCmd, err := c.CmdWrapper(fmt.Sprintf("lxc file pull %s -", filepath.Join(c.ContainerName, src)))
	if err != nil {
		return err
	}

	log.Printf("Running copy command: %s", cpCmd)
	command := ShellCommand(cpCmd)
	command.Stdout = w

	return command.Run()
}

func (c *Communicator) DownloadDir(src string, dst string, exclude []string) error {
	// TODO This could probably be "lxc exec <container> -- cd <src> && tar -czf - | tar -xzf - -C <dst>"
	return fmt.Errorf("DownloadDir is not implemented for lxc")
}

func (c *Communicator) Execute(commandString string) (*exec.Cmd, error) {
	log.Printf("Executing with lxc exec in container: %s %s", c.ContainerName, commandString)
	command, err := c.CmdWrapper(
		fmt.Sprintf("lxc exec %s -- /bin/sh -c \"%s\"", c.ContainerName, commandString))
	if err != nil {
		return nil, err
	}

	localCmd := ShellCommand(command)
	log.Printf("Executing lxc exec: %s %#v", localCmd.Path, localCmd.Args)

	return localCmd, nil
}
