package main

import (
	"flag"
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
		g.frame()
	}
	// FIXME render
}
