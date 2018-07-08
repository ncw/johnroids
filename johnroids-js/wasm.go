// Shim for wasm

// +build js,wasm

package main

import "syscall/js"

type jsObject = js.Value

const (
	isGopherjs = false
	isWasm     = true
	techName   = "go/wasm"
)

var (
	Global      = js.Global()
	newCallback = js.NewCallback
)

func isUndefined(v js.Value) bool {
	return v == js.Undefined()
}

func run(fn func()) {
	// Run now and wait forever
	initialise()
	select {}
}
