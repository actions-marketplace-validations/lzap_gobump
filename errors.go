package main

import "fmt"

const (
	ERR_READ  = 2
	ERR_WRITE = 3
	ERR_PARSE = 4
	ERR_CMD   = 5
	ERR_GIT   = 6
)

var errCmd = fmt.Errorf("command error")
