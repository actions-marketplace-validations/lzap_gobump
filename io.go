package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	ERR_READ  = 2
	ERR_WRITE = 3
	ERR_PARSE = 4
	ERR_CMD   = 5
	ERR_GIT   = 6
)

// cmd runs a subprocess; when verbose, logs the command line, stdout, stderr, and exit code to stderr via Debug.
func cmd(name string, args ...string) error {
	return runCmd(name, args, true)
}

func cmdline(name string, args []string) string {
	if len(args) == 0 {
		return name
	}
	return name + " " + strings.Join(args, " ")
}

func logCmdResult(cmdline string, stdout, stderr *bytes.Buffer, err error) {
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}
	Debug.Printf("exit code: %d\n", exitCode)
	if stdout.Len() > 0 {
		Debug.Println("stdout:\n" + stdout.String())
	}
	if stderr.Len() > 0 {
		Debug.Println("stderr:\n" + stderr.String())
	}
	if err != nil {
		Debug.Println("command failed:", err.Error())
	}
}

func runCmd(name string, args []string, logOutput bool) error {
	line := cmdline(name, args)
	if logOutput && debugEnabled() {
		Debug.Println("running:", line)
	}
	c := exec.Command(name, args...)
	c.Env = os.Environ()
	var stdout, stderr bytes.Buffer
	if logOutput && debugEnabled() {
		c.Stdout = &stdout
		c.Stderr = &stderr
	}
	err := c.Run()
	if logOutput && debugEnabled() {
		logCmdResult(line, &stdout, &stderr, err)
	}
	return err
}

func runCmdOutput(name string, args []string, logOutput bool) ([]byte, error) {
	if !logOutput || !debugEnabled() {
		c := exec.Command(name, args...)
		c.Env = os.Environ()
		return c.Output()
	}
	line := cmdline(name, args)
	Debug.Println("running:", line)
	c := exec.Command(name, args...)
	c.Env = os.Environ()
	var stdout, stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr
	err := c.Run()
	logCmdResult(line, &stdout, &stderr, err)
	return stdout.Bytes(), err
}

var ErrCmd = fmt.Errorf("command error")

func cmds(str string) error {
	parts := strings.Fields(str)
	if len(parts) == 0 {
		return fmt.Errorf("%w: no command", ErrCmd)
	}

	if len(parts) == 1 {
		return cmd(parts[0])
	}

	p1 := parts[0]
	p2 := parts[1:]
	return cmd(p1, p2...)
}
