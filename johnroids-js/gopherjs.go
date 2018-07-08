// Shim for gopherjs

// +build js,!wasm

package main

import "github.com/gopherjs/gopherjs/js"

type jsObject = *js.Object

const (
	isGopherjs = true
	isWasm     = false
	techName   = "gopherjs"
)

var (
	Global = js.Global
)

func isUndefined(v *js.Object) bool {
	return v == nil || v == js.Undefined
}

func newCallback(fn func(args []jsObject)) jsObject {
	return js.MakeFunc(func(this jsObject, args []jsObject) interface{} {
		fn(args)
		return false
	})
}

func run(fn func()) {
	// Run when the dom is loaded
	//document.Call("addEventListener", "DOMContentLoaded", initialise, false)
	initialise()
}
