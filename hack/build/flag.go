package main

import (
	"github.com/spf13/pflag"
)

// the z is a placeholder, the only thing that this cares about is presence
func funczVar(name, usage string, f func() error) {
	flag := pflag.CommandLine.VarPF(funcz(f), name, "", usage)
	flag.NoOptDefVal = "<ignored>" // can't be ""
}

type funcz func() error

func (f funcz) Set(_ string) error {
	return f()
}

func (f funcz) Type() string {
	return "" // funcz?
}

func (f funcz) String() string   { return "" }
func (f funcz) IsBoolFlag() bool { return true }
