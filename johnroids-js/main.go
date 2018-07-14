// Javascript version of johnroids

// +build js

package main

import (
	"fmt"
	"image/color"
	"log"
	"runtime"

	"github.com/ncw/johnroids"
)

// Constants
const (
	scale = 3 // multiply the screen by this much
)

var document jsObject

func getElementById(name string) jsObject {
	node := document.Call("getElementById", name)
	if isUndefined(node) {
		log.Fatalf("Couldn't find element %q", name)
	}
	return node
}

func time() int {
	return Global.Get("Date").New().Call("getTime").Int()
}

// Toggle the visibility of view when button is clicked
func toggle(button, view string) {
	buttonNode := getElementById(button)
	viewNode := getElementById(view)
	buttonNode.Call("addEventListener", "click", newCallback(func(args []jsObject) {
		event := args[0]
		event.Call("preventDefault")
		state := viewNode.Get("style").Get("display").String()
		if state == "none" {
			state = "block"
		} else {
			state = "none"
		}
		viewNode.Get("style").Set("display", state)
	}))
}

func initialise() {
	g := johnroids.New()

	canvas := getElementById("game")
	status := getElementById("status")

	ctx := canvas.Call("getContext", "2d")
	// ctx.Call("scale", scale, scale)

	canvasData := ctx.Call("createImageData", johnroids.SW, johnroids.SH) // filled with transparent black pixels
	data := canvasData.Get("data")

	screen32 := make([]byte, johnroids.SW*johnroids.SH*4)
	var paletteR, paletteG, paletteB [256]byte
	const printInterval = 100
	frameI := 0
	var totalTime int
	var start int

	// Attach key handlers
	for _, key := range []struct {
		id   string
		code johnroids.KeyCode
	}{
		{"key_z", johnroids.KeyCodeZ},
		{"key_x", johnroids.KeyCodeX},
		{"key_shift", johnroids.KeyCodeShift},
		{"key_return", johnroids.KeyCodeReturn},
		{"key_space", johnroids.KeyCodeSpace},
	} {
		node := getElementById(key.id)
		code := key.code
		// Attach mouse and touch handlers for each
		for _, handler := range []struct {
			handlerType string
			pressed     bool
		}{
			{"mousedown", true},
			{"mouseup", false},
			{"touchstart", true},
			{"touchend", false},
		} {
			pressed := handler.pressed
			node.Call("addEventListener", handler.handlerType, newCallback(func(args []jsObject) {
				event := args[0]
				event.Call("preventDefault")
				g.KeyEvent(code, pressed)
			}))
		}
	}

	// Attach button clicks
	toggle("toggle_keyboard", "keyboard")
	toggle("toggle_help", "help")

	plot := func(args []jsObject) {
		start = time()
		screen := g.Frame()

		for i, c := range screen.Palette {
			cc := c.(color.RGBA)
			paletteR[i] = cc.R
			paletteG[i] = cc.G
			paletteB[i] = cc.B
		}

		i := 0
		for _, c := range screen.Pix {
			screen32[i+0] = paletteR[c] // R
			screen32[i+1] = paletteG[c] // G
			screen32[i+2] = paletteB[c] // B
			screen32[i+3] = 0xFF        // A
			i += 4
		}

		data.Call("set", screen32)

		ctx.Call("putImageData", canvasData, 0, 0)

		dt := time() - start
		totalTime += dt
		frameI++
		if frameI >= printInterval {
			status.Set("innerHTML", fmt.Sprintf("%s: plot time %.3fms", techName, float64(totalTime)/printInterval))
			frameI = 0
			totalTime = 0
		}
	}
	Global.Call("setInterval", newCallback(plot), johnroids.MinMsPerFrame)

	keyEvent := func(args []jsObject) {
		event := args[0]
		//event.Call("preventDefault")
		key := event.Get("key").String()
		eventType := event.Get("type").String()
		pressed := eventType == "keydown"
		//log.Printf("Key %q pressed %v", key, pressed)
		switch key {
		case "z", "Z":
			g.KeyEvent(johnroids.KeyCodeZ, pressed)
		case "x", "X":
			g.KeyEvent(johnroids.KeyCodeX, pressed)
		case "Shift":
			g.KeyEvent(johnroids.KeyCodeShift, pressed)
		case "Enter":
			g.KeyEvent(johnroids.KeyCodeReturn, pressed)
		case " ":
			g.KeyEvent(johnroids.KeyCodeSpace, pressed)
		case "Escape":
			fmt.Printf("Escape pressed - bye\n")
		}
	}
	document.Call("addEventListener", "keydown", newCallback(keyEvent))
	document.Call("addEventListener", "keyup", newCallback(keyEvent))

	status.Set("innerHTML", fmt.Sprintf("%s: warming up", techName))
}

func main() {
	log.Printf("Running on goos/goarch = %s/%s", runtime.GOOS, runtime.GOARCH)
	if isUndefined(Global) {
		log.Fatalf("Didn't find Global - not running in browser")
	}
	document = Global.Get("document")
	if isUndefined(document) {
		log.Fatalf("Didn't find document - not running in browser")
	}
	run(initialise)
}
