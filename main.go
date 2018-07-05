package main

import (
	"flag"
	"fmt"
	"image/color"
	"log"
	"os"
	"runtime/pprof"

	"github.com/veandco/go-sdl2/sdl"
)

// Globals
var (
	renderer *sdl.Renderer
)

// Initialise the screen returning a function to finalize it
func screen_initialise() func() {
	// Initialize the SDL library
	if err := sdl.Init(sdl.INIT_VIDEO); err != nil {
		die("Couldn't initialize SDL: %v", err)
	}

	window, err := sdl.CreateWindow("JohnRoids", sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED, SW*scale, SH*scale, sdl.WINDOW_SHOWN)
	if err != nil {
		panic(err)
	}

	renderer, err = sdl.CreateRenderer(window, -1, sdl.RENDERER_ACCELERATED)
	if err != nil {
		die("Couldn't initialize renderer: %v", err)
	}
	renderer.SetScale(scale, scale)

	surface, err := window.GetSurface()
	if err != nil {
		die("Couldn't initialize surface: %v", err)
	}
	surface.FillRect(nil, 0)

	return func() {
		renderer.Destroy()
		window.Destroy()
		sdl.Quit()
	}
}

// Read the keys
func readEvents(g *Game) {
	event := sdl.PollEvent()
	switch x := event.(type) {
	case nil:
	case *sdl.KeyboardEvent:
		pressed := x.State != 0
		switch x.Keysym.Scancode {
		case sdl.SCANCODE_Z:
			g.KeyEvent(KeyCodeZ, pressed)
		case sdl.SCANCODE_X:
			g.KeyEvent(KeyCodeX, pressed)
		case sdl.SCANCODE_RSHIFT, sdl.SCANCODE_LSHIFT:
			g.KeyEvent(KeyCodeShift, pressed)
		case sdl.SCANCODE_RETURN:
			g.KeyEvent(KeyCodeReturn, pressed)
		case sdl.SCANCODE_SPACE:
			g.KeyEvent(KeyCodeSpace, pressed)
		case sdl.SCANCODE_ESCAPE:
			fmt.Printf("Escape pressed - bye\n")
			os.Exit(0)
		}
	case *sdl.QuitEvent:
		fmt.Printf("SDL quit received - bye\n")
		os.Exit(0)
	}
}

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")

func main() {
	defer screen_initialise()()
	flag.Parse()
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}
	g := New()
	for {
		readEvents(g)
		screen := g.frame()

		// Clear the screen
		renderer.SetDrawColor(0, 0, 0, 255)
		renderer.Clear()

		// super simple image conversion
		for y := 0; y < screen.Rect.Dy(); y += 1 {
			for x := 0; x < screen.Rect.Dx(); x += 1 {
				c := screen.ColorIndexAt(x, y)
				if c != 0 {
					if int(c) >= len(g.palette) {
						debugf("color out of range %d/%d", c, len(g.palette))
						c = uint8(len(g.palette) - 1)
					}
					cc := screen.Palette[c].(color.RGBA)
					renderer.SetDrawColor(cc.R, cc.G, cc.B, cc.A)
					renderer.DrawPoint(int32(x), int32(y))
				}
			}
		}

		// show the changes
		renderer.Present()

	}
	// FIXME render
}
