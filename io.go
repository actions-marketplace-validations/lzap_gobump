package main

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Cmd runs a subprocess; when verbose, logs the command line, stdout, stderr, and exit code to stderr via Debug.
func Cmd(name string, args ...string) error {
	return RunCmd(name, args, true)
}

func Cmdline(name string, args []string) string {
	if len(args) == 0 {
		return name
	}
	return name + " " + strings.Join(args, " ")
}

func LogCmdResult(line string, stdout, stderr *bytes.Buffer, err error) {
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

func RunCmd(name string, args []string, logOutput bool) error {
	line := Cmdline(name, args)
	if logOutput && DebugEnabled() {
		Debug.Println("running:", line)
	}
	c := exec.Command(name, args...)
	c.Env = subprocessEnv(name)
	var stdout, stderr bytes.Buffer
	if logOutput && DebugEnabled() {
		c.Stdout = &stdout
		c.Stderr = &stderr
	}
	err := c.Run()
	if logOutput && DebugEnabled() {
		LogCmdResult(line, &stdout, &stderr, err)
	}
	return err
}

func RunCmdOutput(name string, args []string, logOutput bool) ([]byte, error) {
	if !logOutput || !DebugEnabled() {
		c := exec.Command(name, args...)
		c.Env = subprocessEnv(name)
		return c.Output()
	}
	line := Cmdline(name, args)
	Debug.Println("running:", line)
	c := exec.Command(name, args...)
	c.Env = subprocessEnv(name)
	var stdout, stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr
	err := c.Run()
	LogCmdResult(line, &stdout, &stderr, err)
	return stdout.Bytes(), err
}

func Cmds(str string) error {
	parts := strings.Fields(str)
	if len(parts) == 0 {
		return fmt.Errorf("%w: no command", ErrCmd)
	}

	if len(parts) == 1 {
		return Cmd(parts[0])
	}

	p1 := parts[0]
	p2 := parts[1:]
	return Cmd(p1, p2...)
}
