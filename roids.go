/*
JohnRoids - the conversion!

note that the error checking for object table full has been taken
out...

make a rand_chance (x/y) function

Note that object 0 is the ship

Speed goes wrong when we have the ghostship countdown

Need to figure out what happens on gameover!  gamepause should be in
main loop really...

Try making an explode image function which makes an object per pixel
in the object and gives then the right velocities to make an
explosion.  Could then have a fade table for each of 256 pixels.  Will
run out of objects pretty quickly!  Perhaps only want to explode every
10th pixel in the roids or something like that.  Could record the x
and y of the hit and use that as the centre of the explosion.

Could reduce the width and height of the sprite by removing all the
black rows at the top and the bottom left and right.  This would speed
up the plotter and enable us to calculate the c of g so we could split
the roids so that the bits always came apart from the centre.

Could make the explosion always start from the bullet hit - this might
be quite realistic.
*/
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"math/rand"
	"os"
	"time"

	"github.com/veandco/go-sdl2/sdl"
)

const (
	SW                  = 320
	SH                  = 256
	CHECK_PLOT          = true
	pixelshift          = 8
	logshipfriction     = 8
	maxobjects          = 4096 // must be a power of two
	maxsprites          = 64
	MIN_MS_PER_FRAME    = 20
	fade_colours        = 256
	LOG_COS_TABLE_SCALE = 16
	COS_TABLE_SCALE     = (1 << LOG_COS_TABLE_SCALE)
	scale               = 3
)

// return the current ship direction
func ship_direction() int {
	return object[0].state
}

// clip *z into the range 0 <= *z < w
func CLIP(z *int, w int) {
	if *z < 0 {
		*z += w
	}
	if *z >= w {
		*z -= w
	}
}

// structures

type object_block struct {
	typ   int
	state int
	life  int // life in cs
	x     int
	y     int
	dx    int
	dy    int
	phase int
	lives int
}

// static variables

var (
	screen       *image.Paletted
	palette      color.Palette
	foundPalette = map[rgba]uint8{}
	renderer     *sdl.Renderer
	sprite       [maxsprites]*image.Paletted

	object [maxobjects]object_block

	fade_colour [fade_colours][16]uint8

	time_now      int
	dtime         int
	cos_table     [16]int
	sin_table     [16]int
	cos_table2    [256]int
	sin_table2    [256]int
	hit           int
	hit_x         int
	hit_y         int
	collision     int
	collision_x   int
	collision_y   int
	key_time      int
	nroids        int
	lives         int
	level         int
	score         int
	hscore        int
	ghostship     bool
	spacepressed  bool
	escapepressed bool
	bullet_colour uint8

	slives   = make([]*object_block, 5)
	sscore   = make([]*object_block, 6)
	shscore  = make([]*object_block, 6)
	slevel   = make([]*object_block, 2)
	sframems = make([]*object_block, 3)

	start_update_time int
	plot_time         int // time it takes to plot a frame in ms

	// sprite numbers

	spritepointer, null, ship, roid, roid_endsplit, roid_endsmall  int
	roid_attacker, roid_shooter, roid_weapon, bullet, pretty, dust int
	levelword, scoreword, highscoreword, johnroidsword, showlife   int
	numbers, instructionsword, gameoverword                        int
)

// Write some debug stuff
func debugf(format string, a ...interface{}) {
	fmt.Printf(format+"\n", a...)
}

// Blow up with a fatal error
func die(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
	// va_list va;
	// char *error;

	// va_start(va, format_string);
	// vfprintf(stderr, format_string, va);
	// va_end(va);

	// error = sdl.GetError();
	// if (error && *error)
	//     fprintf(stderr, "SDL error: '%s'\n", error);
	// if (errno)
	//     fprintf(stderr, "OS error : '%s'\n", strerror(errno));

	// exit(EXIT_FAILURE);
	os.Exit(1)
}

// compact color representation
type rgba struct {
	r, g, b, a uint8
}

// Converts a color.Color into an rgba
func newRGBA(c color.Color) rgba {
	r, g, b, a := c.RGBA()
	return rgba{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
}

func screen_initialise() func() {
	// Initialize the SDL library
	if err := sdl.Init(sdl.INIT_VIDEO); err != nil {
		die("Couldn't initialize SDL: %v", err)
	}

	/*
		// Initialize the display in a SWxSH 8-bit palettized mode
		screen = sdl.SetVideoMode(SW, SH, 8, sdl.SWSURFACE|sdl.DOUBLEBUF)
		//    screen = sdl.SetVideoMode(SW, SH, 8, sdl.FULLSCREEN | sdl.DOUBLEBUF);
		if screen == nil {
			die("Couldn't set %ix%ix8 video mode: %s\n", SW, SH, sdl.GetError())
		}

		sdl.WM_SetCaption("JohnRoids", "JohnRoids")
	*/
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

// This finds a space in the objects, returning the address of that
// space in r0, or 0 if there was no space
func findspace() *object_block {
	o := rand.Int() & (maxobjects - 1)
	if o == 0 { // don't use the ship (first) slot
		o++
	}

	for i := maxobjects - 10; i > 0; i-- { // try maxobjects-10 slots
		if object[o].typ == 0 {
			return &object[o]
		}
		o++
		if o >= maxobjects {
			o = 1
		}
	}

	debugf("Out of objects\n")
	return nil
}

func clearobject(obj *object_block) {
	obj.typ = 0
	obj.state = 0
	obj.phase = 0
	obj.lives = 0
	obj.life = 0
	obj.dx = 0
	obj.dy = 0
	obj.x = 0
	obj.y = 0
}

// This finds a space in the objects, returning the address of that
// space in r0, or 0 if there was no space.
// It also zeros all the entries in that object
func findspacenzero() *object_block {
	obj := findspace()
	if obj == nil {
		return nil
	}

	clearobject(obj)

	return obj
}

// writes a png from the image given
func writeImage(img *image.Paletted, path string) {
	out, err := os.Create(path)
	if err != nil {
		die("Failed to open image for write %s: %v", path, err)
	}
	defer func() {
		err = out.Close()
		if err != nil {
			die("Failed to close image for write %s: %v", path, err)
		}
	}()

	err = png.Encode(out, img)
	if err != nil {
		die("Failed to encode image %s: %v", path, err)
	}

	debugf("Written image %s", path)
}

// Convert a gray scale image to a paletted one
func grayToPaletted(gray *image.Gray) *image.Paletted {
	var used [256]bool
	for y := 0; y < gray.Rect.Dy(); y++ {
		for x := 0; x < gray.Rect.Dx(); x++ {
			px := gray.GrayAt(x, y)
			used[px.Y] = true
		}
	}
	var p color.Palette
	for y, found := range used {
		if found {
			Y := uint8(y)
			p = append(p, color.RGBA{Y, Y, Y, 255})
		}
	}
	img := image.NewPaletted(gray.Rect, p)
	for y := 0; y < gray.Rect.Dy(); y++ {
		for x := 0; x < gray.Rect.Dx(); x++ {
			px := gray.At(x, y)
			img.Set(x, y, px)
		}
	}
	return img
}

// Re-order the image palette and the image data to match the standard one
func reorderPalette(img *image.Paletted) {
	var pixmap [256]uint8
	// Read the colors out of the palette
	for i, c := range img.Palette {
		pixmap[i] = foundPalette[newRGBA(c)]
	}
	// swizzle the image data
	for i := range img.Pix {
		img.Pix[i] = pixmap[img.Pix[i]]
	}
	// replace the pallete with the standard one
	img.Palette = palette
}

// Make palette from the loaded values then make the screen
func make_screen() {
	// black must be the first entry
	palette = append(palette, color.RGBA{0, 0, 0, 255})
	black := rgba{0, 0, 0, 255}
	foundPalette[black] = 0
	for c := range foundPalette {
		if c == black {
			continue
		}
		r, b, g, a := c.r, c.b, c.g, c.a
		palette = append(palette, color.RGBA{r, g, b, a})
		foundPalette[c] = uint8(len(palette) - 1)
	}
	debugf("Found %d colors", len(palette))
	if len(palette) > 256 {
		die("too many colors")
	}

	// rewrite the palettes of all the sprites
	for i := 0; i < spritepointer; i++ {
		reorderPalette(sprite[i])
	}

	// work out what the bullet_colour is
	b := sprite[bullet]
	for x := 0; x < b.Rect.Dx(); x++ {
		for y := 0; y < b.Rect.Dy(); y++ {
			p := b.ColorIndexAt(x, y)
			if p != 0 {
				bullet_colour = p
			}
		}
	}

	// Now we've loaded all the sprites we can make the screen
	screen = image.NewPaletted(image.Rect(0, 0, SW, SH), palette)

	// make the fade colours
	for i, col := range palette {
		r, g, b, a := col.RGBA()
		r >>= 8
		g >>= 8
		b >>= 8
		a >>= 8
		// debugf("r,g,b = %d,%d,%d", r, g, b)
		for j := uint32(0); j < 16; j++ {
			k := 16 - j
			r, g, b := (r*k)/16, (g*k)/16, (b*k)/16
			paletteCol := palette.Convert(color.RGBA{uint8(r), uint8(g), uint8(b), 255})
			pixel := foundPalette[newRGBA(paletteCol)]
			// debugf("fade_colour[%d][%d]=%d (%d,%d,%d)", i, j, pixel, (r*k)/16, (g*k)/16, (b*k)/16)
			fade_colour[i][j] = pixel
		}
	}
}

// Load sprite
func load_sprite(name string) int {
	path := fmt.Sprintf("images/%s.png", name)
	// debugf("Loading sprite '%s' from '%s'", name, path)
	fd, err := os.Open(path)
	if err != nil {
		die("Failed to open image %s: %v", path, err)
	}
	defer fd.Close()

	goImage, _, err := image.Decode(fd)
	if err != nil {
		die("Failed to decode image %s: %v", path, err)
	}
	var palettedImage *image.Paletted
	switch x := goImage.(type) {
	case *image.Gray:
		palettedImage = grayToPaletted(x)
	case *image.Paletted:
		palettedImage = x
	default:
		die("Can't load image %s of type %T", path, goImage)
	}
	sprite[spritepointer] = palettedImage

	// Read the colors out of the palette
	for _, c := range palettedImage.Palette {
		foundPalette[newRGBA(c)] = 0
	}

	if spritepointer >= maxsprites {
		die("Too many sprites")
	}

	// // Load the BMP file into a surface
	// sdlImage, err := img.Load(path)
	// if err != nil {
	// 	die("Couldn't load sprite %s: %v", path, err)
	// }
	// sprite[spritepointer] = sdlImage

	// // Set the colour key for blitting
	// sdlImage.SetColorKey(true, 0)

	n := spritepointer
	spritepointer++
	return n
}

// Load sprites
func load_sprites() {
	spritepointer = 0

	null = spritepointer
	load_sprite("0")

	ship = spritepointer
	for i := 0; i <= 15; i++ {
		ship_name := fmt.Sprintf("ship%d", i)
		load_sprite(ship_name)
	}

	// roids split into n -> ( 2n+1, 2n+2 )
	roid = spritepointer
	load_sprite("0")   // 0 -> 1,2
	load_sprite("00")  // 1 -> 3,4
	load_sprite("01")  // 2 -> 5,6
	load_sprite("000") // 3 -> 7,8
	load_sprite("001") // 4 -> 9,10
	load_sprite("010") // 5 -> 11,12
	load_sprite("011") // 6 -> 13,14
	roid_endsplit = spritepointer - 1 - roid
	load_sprite("0000") // 7
	load_sprite("0001") // 8
	load_sprite("0010") // 9
	load_sprite("0011") // 10
	load_sprite("0100") // 11
	load_sprite("0101") // 12
	load_sprite("0110") // 13
	load_sprite("0111") // 14
	roid_endsmall = spritepointer - 1 - roid

	roid_attacker = spritepointer - roid
	load_sprite("attacker")

	roid_shooter = spritepointer - roid
	load_sprite("shooter")

	roid_weapon = spritepointer - roid
	load_sprite("weapon0")
	load_sprite("weapon1")
	load_sprite("weapon2")
	load_sprite("weapon3")

	bullet = spritepointer
	load_sprite("missile")

	// all sprites after here are just for effect

	pretty = spritepointer

	dust = spritepointer
	load_sprite("dust")

	levelword = spritepointer
	load_sprite("level")

	scoreword = spritepointer
	load_sprite("score")

	highscoreword = spritepointer
	load_sprite("highscore")

	johnroidsword = spritepointer
	load_sprite("johnroids")

	showlife = spritepointer
	load_sprite("ship4")

	numbers = spritepointer
	load_sprite("n0")
	load_sprite("n1")
	load_sprite("n2")
	load_sprite("n3")
	load_sprite("n4")
	load_sprite("n5")
	load_sprite("n6")
	load_sprite("n7")
	load_sprite("n8")
	load_sprite("n9")

	instructionsword = spritepointer
	load_sprite("instrs")

	gameoverword = spritepointer
	load_sprite("gameover")
}

var start_time = time.Now()

// ticks since the start of the program in ms
func get_ticks() int {
	return int(time.Since(start_time) / time.Millisecond)
}

// Read the time in 1/100 of a second
func read_time() int {
	return get_ticks() / 10
}

func init_vars() {
	rand.Seed(time.Now().UTC().UnixNano())

	for i := 0; i < 16; i++ {
		cos_table[i] = int(COS_TABLE_SCALE*math.Cos(float64(i)/16*2*math.Pi) + 0.5)
		sin_table[i] = int(COS_TABLE_SCALE*math.Sin(-float64(i)/16*2*math.Pi) + 0.5)
	}

	for i := 0; i < 256; i++ {
		cos_table2[i] = int(COS_TABLE_SCALE*math.Cos(float64(i)/256*2*math.Pi) + 0.5)
		sin_table2[i] = int(COS_TABLE_SCALE*math.Sin(-float64(i)/256*2*math.Pi) + 0.5)
	}
}

// This sets up all the objects needed at the start of a game
// This includes the score, highscore, and the lives.
// It removes any previous objects
func setupobjects() {
	for i := 0; i < maxobjects; i++ {
		clearobject(&object[i])
	}

	lives = 3

	ghostship = false
	score = 0
	level = 0
	nroids = 0

	object[0].typ = ship
	object[0].state = 0
	object[0].phase = 0
	object[0].lives = 0
	object[0].life = 0
	object[0].dx = 0
	object[0].dy = 0
	object[0].x = (SW / 2) << pixelshift
	object[0].y = (SH / 2) << pixelshift

	obj := findspacenzero()
	obj.typ = scoreword

	x := int(48) << pixelshift
	for i := 0; i < 6; i++ {
		obj = findspacenzero()
		sscore[i] = obj
		obj.typ = numbers
		obj.x = x
		x += 8 << pixelshift
	}

	obj = findspacenzero()
	obj.typ = levelword
	obj.x = (SW/2 - 46) << pixelshift // 114<<pixelshift;

	slevel[0] = findspacenzero()
	slevel[0].typ = numbers
	slevel[0].x = (SW/2 - 2) << pixelshift // 158<<pixelshift;

	slevel[1] = findspacenzero()
	slevel[1].typ = numbers
	slevel[1].x = (SW/2 + 6) << pixelshift // 166<<pixelshift;

	x = 0 << pixelshift
	for i := 0; i < 5; i++ { // can have up to 5 lives
		obj = findspacenzero()
		slives[i] = obj
		if i > 3 {
			obj.typ = 0
		} else {
			obj.typ = showlife // 3 lives to start with
		}

		obj.y = (SH - 12) << pixelshift
		obj.x = x
		x += 8 << pixelshift
	}

	obj = findspacenzero()
	obj.typ = highscoreword
	obj.x = (SW - 132) << pixelshift

	x = (SW - 48) << pixelshift // 272
	for i := 0; i < 6; i++ {
		obj = findspacenzero()
		shscore[i] = obj
		obj.typ = numbers
		obj.x = x
		x += 8 << pixelshift
	}

	x = (SW - 24) << pixelshift
	for i := 0; i < 3; i++ {
		obj = findspacenzero()
		sframems[i] = obj
		obj.typ = numbers
		obj.y = (SH - 12) << pixelshift
		obj.x = x
		x += 8 << pixelshift
	}
}

// This converts the number in r0, into r1 decimal digits, putting
// the results into the objects pointed to by the array of pointers pointed
// to by r2
func makedigits(n int, output []*object_block) {
	digitbuffer := fmt.Sprintf("%0*d", len(output), n)
	for i, c := range digitbuffer {
		output[i].state = int(c - '0')
	}
}

// This fires a bullet from a roid, pointed to in r0
func roidfire(parent *object_block) {
	weapon := findspace()

	weapon.typ = roid
	weapon.state = roid_weapon
	weapon.life = 0
	weapon.lives = 0
	weapon.phase = (rand.Int() & 3) | (0x03 << 8)
	weapon.x = parent.x
	weapon.y = parent.y

	// calculate a trajectory to fire at the ship
	dx := object[0].x - weapon.x
	dy := object[0].y - weapon.y
	abs_dx := dx
	if dx < 0 {
		abs_dx = -dx
	}
	abs_dy := dy
	if dy < 0 {
		abs_dy = -dy
	}

	// shift dx,dy until less than a certain amount so bullet doesn't
	// move too fast.  normal programs would use a divide here but
	// remember this is an assembler conversion ;-)
	for abs_dx >= (1<<pixelshift) || abs_dy >= (1<<pixelshift) {
		dx /= 2
		dy /= 2
		abs_dx /= 2
		abs_dy /= 2
	}

	weapon.dx = dx
	weapon.dy = dy

	// sound_roidfire()

	nroids++
}

// This fires a bullet from the ship
func fire() {
	obj := findspace()

	score--
	if score < 0 {
		score = 0
	}

	obj.typ = bullet
	obj.state = 0
	obj.phase = 0
	obj.lives = 0

	obj.life = time_now + 200

	x := object[0].x
	y := object[0].y
	dx := object[0].dx
	dy := object[0].dy

	c := cos_table[ship_direction()]
	s := sin_table[ship_direction()]

	x += ((16 - 4) / 2) << pixelshift
	y += ((16 - 4) / 2) << pixelshift
	x += c / (1 << (13 - pixelshift))
	x += c / (1 << (15 - pixelshift))
	y += s / (1 << (13 - pixelshift))
	y += s / (1 << (15 - pixelshift))
	obj.x = x
	obj.y = y

	dx += c / (1 << (16 - pixelshift))
	dy += s / (1 << (16 - pixelshift))
	obj.dx = dx
	obj.dy = dy

	// sound_fire();
}

// This blows up the roid pointed to by r0 into 2 smaller bits
// or into nothing, depending on size
func blowroid(r *object_block, x int, y int) {
	score += r.state * 10

	if r.state > roid_endsplit {
		// don't split this roid into two
		if r.state > roid_endsmall || (rand.Int()&0xFF) > (0x100/10) {
			r.typ = 0 // destroy roid
			nroids--
		} else {
			// make an attacking roid if small roid and chance
			r.state = roid_attacker
			r.x = x - (8 << pixelshift)
			r.y = y - (8 << pixelshift)
			// debugf("roid_attacker at (%g,%g)\n", (double)x/(1<<pixelshift), (double)y/(1<<pixelshift));
		}
	} else {
		if r.state == 0 && (rand.Int()&0xFF) < (0x100/2) {
			// make a shooting roid if big roid and chance
			r.state = roid_shooter
			r.x += ((32 - 24) / 2) << pixelshift
			r.y += ((32 - 24) / 2) << pixelshift
			roidfire(r) // make it fire
		} else {
			s := findspace()

			// split the roid into two
			r.state = 2*r.state + 1
			s.typ = roid
			s.state = r.state + 1
			s.life = 0
			s.lives = 0
			s.phase = 0
			s.x = r.x
			s.y = r.y

			// Add a random element to the speed to make the roids split
			dx := (rand.Int() & 0xFF) - 0x80
			dy := (rand.Int() & 0xFF) - 0x80
			s.dx = r.dx - dx
			s.dy = r.dy - dy
			r.dx = r.dx + dx
			r.dy = r.dy + dy

			nroids++
		}
	}
}

// This explodes an object into its component pixels with a blast centre
// of (x,y).  Use (-1,-1) to use the centre of the object.
func explode_object(obj *object_block, x int, y int, particles int, mean_speed int, speed_mask int, life_mask int) {
	image := sprite[obj.typ+obj.state+(obj.phase&0xFF)]
	//      int dx = obj.dx;
	//      int dy = obj.dy;
	dangle := 0x10000 / particles
	angle := rand.Int() % dangle

	// use the centre of the image if no center specified
	if x == -1 && y == -1 {
		x = obj.x + (image.Rect.Dx() << (pixelshift - 1))
		y = obj.y + (image.Rect.Dy() << (pixelshift - 1))
	}

	for i := int(0); i < image.Rect.Dx(); i++ {
		for j := int(0); j < image.Rect.Dy(); j++ {
			colour := image.ColorIndexAt(i, j)
			if colour != 0 {
				p := findspace()
				p.typ = dust
				p.state = int(colour)
				p.life = time_now + (rand.Int() & life_mask)
				p.x = obj.x + (i << pixelshift) + (rand.Int() & 0x1F) - 0x10
				p.y = obj.y + (j << pixelshift) + (rand.Int() & 0x1F) - 0x10
				speed := mean_speed
				if speed_mask != 0 {
					speed -= (rand.Int() & speed_mask)
				}
				//p.dx = dx + ( cos_table2[angle >> 8] * speed ) / COS_TABLE_SCALE;
				//p.dy = dy + ( sin_table2[angle >> 8] * speed ) / COS_TABLE_SCALE;
				//p.dx = dx + (rand.Int() & 0xFF) - 0x80;
				//p.dy = dy + (rand.Int() & 0xFF) - 0x80;
				p.dx = obj.dx + (i << (pixelshift - 3)) - (image.Rect.Dx() << (pixelshift - 4))
				p.dy = obj.dy + (j << (pixelshift - 3)) - (image.Rect.Dy() << (pixelshift - 4))
				angle += dangle
			}
		}
	}

	//  for (; particles > 0; particles--)
	//  {
	//      object_block *p = findspace();
	//      p.typ = dust;
	//      p.state = colour_row < 0 ? (rand.Int() % fade_colours) : colour_row;
	//      //p.life = time_now + (rand.Int() & life_mask);
	//      p.life = time_now + life_mask;
	//      p.x = x;
	//      p.y = y;
	//      //angle = rand.Int() & 0xFF;
	//      speed = mean_speed;
	//      if (speed_mask)
	//          speed -= (rand.Int() & speed_mask);
	//      p.dx = dx + ( cos_table2[angle >> 8] * speed ) / COS_TABLE_SCALE;
	//      p.dy = dy + ( sin_table2[angle >> 8] * speed ) / COS_TABLE_SCALE;
	//      //p.dx = dx + (rand.Int() & 0xFF) - 0x80;
	//      //p.dy = dy + (rand.Int() & 0xFF) - 0x80;
	//      angle += dangle;
	// }
}

// This makes an explosion at the object pointed to by r0
// with r1 particles, of colour_row r2, or random if r2<0
// r3 is used to mask centi-seconds of life
func explosion(obj *object_block, x int, y int, particles int, mean_speed int, speed_mask int, colour_row int, life_mask int) {
	image := sprite[obj.typ+obj.state+(obj.phase&0xFF)]
	dx := obj.dx
	dy := obj.dy
	dangle := 0x10000 / particles
	angle := rand.Int() % dangle

	if x < 0 {
		x = obj.x + (image.Rect.Dx() << (pixelshift - 1))
	}
	if y < 0 {
		y = obj.y + (image.Rect.Dy() << (pixelshift - 1))
	}

	for ; particles > 0; particles-- {
		p := findspace()
		p.typ = dust
		p.state = colour_row
		if colour_row < 0 {
			p.state = (rand.Int() % fade_colours)
		}
		// p.life = time_now + (rand.Int() & life_mask);
		p.life = time_now + life_mask
		p.x = x
		p.y = y
		angle = rand.Int() & 0xFFFF
		// angle += dangle;
		speed := mean_speed
		if speed_mask != 0 {
			speed -= (rand.Int() & speed_mask)
		}
		p.dx = dx + (cos_table2[angle>>8]*speed)/COS_TABLE_SCALE
		p.dy = dy + (sin_table2[angle>>8]*speed)/COS_TABLE_SCALE
		// p.dx = dx + (rand.Int() & 0xFF) - 0x80;
		// p.dy = dy + (rand.Int() & 0xFF) - 0x80;
	}
}

// This blows up the ship
func blowship(x int, y int) {
	// sound_blowship()
	explode_object(&object[0], x, y, 64, 0x100, 0x7F, 0x1FF)
	explosion(&object[0], x, y, 64, 0x80, 0x1F, -1, 0x1FF)
	object[0].typ = 0
}

// This explodes a bullet that has hit, or is time expired
func bulletexplosion(obj *object_block) {
	explode_object(obj, -1, -1, 8, 0x20, 0x00, 0x7F)
	//    explosion(obj, 8, 0x20, 0x00, bullet_colour, 0x7F);
	// sound_bulletexplosion();
}

// This explodes a roid
func roidexplosion(obj *object_block, x int, y int) {
	//    explode_object(obj, x, y, 32, 0x80, 0x7F, 0xFF);
	explosion(obj, x, y, 32, 0x80, 0x7F, rand.Int()%fade_colours, 0xFF)
	// sound_roidexplosion();
}

// This updates the co-ordinates of the object in r0 co-ordinates
func update(obj *object_block) {
	sy := obj.y
	sx := obj.x

	sx = dtime*obj.dx + sx // scale for time
	CLIP(&sx, SW<<pixelshift)
	obj.x = sx // x+=dx

	sy = dtime*obj.dy + sy // scale for time
	CLIP(&sy, SH<<pixelshift)
	obj.y = sy // y+=dy

	// if object is mortal and over age limit, kill it
	if obj.life != 0 && obj.life-time_now <= 0 {
		// debugf("Killing object %i %i %i\n", obj.life, time_now, obj.life - time_now);
		if obj.typ == bullet {
			bulletexplosion(obj)
		}
		obj.typ = 0
		// return; FIXME ;;;; ???
	}

	// ldr   r1,[r0.phase]
	// add   r2,r1,#1
	// and   r2,r2,r1,lsr#8
	// and   r1,r1,#&FF00
	// orr   r1,r1,r2
	// str   r1,[r0.phase]         |* increment object phase *|

	// r1 = obj.phase;
	// r2 = r1 + 1;
	// r2 = r2 & (r1 >> 8);
	// r1 = r1 & 0xFF00;
	// r1 = r1 | r2;
	// obj.phase = r1;             |* increment object phase *|

	// increment object phase
	obj.phase = (obj.phase & 0xFF00) | ((obj.phase + 1) & (obj.phase >> 8))

	// if ship is dead, don't do the below
	if object[0].typ != ship {
		return
	}

	// These deal with attacking the ship

	// Drive the attacker in decaying orbits around the ship
	if obj.typ == roid && obj.state == roid_attacker {
		dx := obj.dx
		dy := obj.dy

		dx -= (dx / (1 << 10))
		if object[0].x > obj.x {
			dx += 4
		}
		if object[0].x < obj.x {
			dx -= 4
		}
		obj.dx = dx

		dy -= (dy / (1 << 10))
		if object[0].y > obj.y {
			dy += 4
		}
		if object[0].y < obj.y {
			dy -= 4
		}
		obj.dy = dy
	}

	// make sure the shooter shoots now and again
	if obj.typ == roid && obj.state == roid_shooter &&
		!ghostship && (rand.Int()&0xFF) < (0xFF/20) {
		roidfire(obj)
	}
}

// Plot a pixel of a given colour directly to the screen
func plot_pixel(X int, Y int, pixel uint8) {
	if X < 0 || X >= screen.Rect.Dx() || Y < 0 || Y >= screen.Rect.Dy() {
		debugf("Out of bounds pixel %i,%i", X, Y)
		return
	}

	screen.SetColorIndex(X, Y, pixel)

	// Map the color yellow to this display (R=0xFF, G=0xFF, B=0x00)
	// Note:  If the display is palettized, you must set the palette first.
	//    pixel = sdl.MapRGB(screen.format, 0xFF, 0xFF, 0x00);

	// Calculate the framebuffer offset of the center of the screen
	// if screen.MustLock() {
	// 	if err := screen.Lock(); err != nil {
	// 		die("Couldn't lock screen: %v", err)
	// 	}
	// }
	// bpp := screen.Format.BytesPerPixel
	// FIXME bits := screen.Pixels()[Y*screen.Pitch+X*int(bpp)]

	// Set the pixel
	// switch bpp {
	// case 1:
	// 	// FIXME *((byte *)(bits)) = (byte)pixel;
	// case 2:
	// 	// FIXME *((Uint16 *)(bits)) = (Uint16)pixel;
	// case 3:
	// 	// Format/endian independent
	// 	//var r, g, b byte

	// 	// FIXME
	// 	// r = (pixel >> screen.Format.Rshift) & 0xFF
	// 	// g = (pixel >> screen.Format.Gshift) & 0xFF
	// 	// b = (pixel >> screen.Format.Bshift) & 0xFF
	// 	// *((bits)+screen.format.Rshift/8) = r;
	// 	// *((bits)+screen.format.Gshift/8) = g;
	// 	// *((bits)+screen.format.Bshift/8) = b;
	// case 4:
	// 	// FIXME *((uint *)(bits)) = (uint)pixel;
	// }

	// // Update the display
	// if screen.MustLock() {
	// 	screen.Unlock()
	// }
}

// this plots the object whose address is in r0
func plot(obj *object_block) {
	hit = 0
	collision = 0

	// if true { // FIXME
	// 	var rect sdl.Rect
	// 	if obj.typ != dust {
	// 		image := sprite[obj.typ+obj.state+(obj.phase&0xFF)]
	// 		rect.X = obj.x >> pixelshift
	// 		rect.Y = obj.y >> pixelshift
	// 		rect.W = image.Rect.Dx()
	// 		rect.H = image.Rect.Dy()

	// 		texture, err := renderer.CreateTextureFromSurface(image)
	// 		if err != nil {
	// 			die("Create texture from surface error: %v", err)
	// 		}
	// 		defer texture.Destroy()

	// 		renderer.Copy(texture, nil, &rect)

	// 		// err := image.Blit(nil, screen, &rect)
	// 		// if err != nil {
	// 		// 	die("Blit surface error: %v", err)
	// 		// }

	// 		// TEST
	// 		if (rand.Int() & 0xFF) == 0 {
	// 			hit = 1
	// 		}
	// 		if (rand.Int() & 0xFF) == 0 {
	// 			collision = 1
	// 		}
	// 	} else if obj.typ == dust {
	// 		fade := (obj.life - time_now) >> 2
	// 		fade = 15 - fade
	// 		if fade > 15 {
	// 			fade = 15
	// 		}
	// 		if fade < 0 {
	// 			fade = 0
	// 		}
	// 		col := fade_colour_rgb[obj.state][fade]
	// 		renderer.SetDrawColor(col.r, col.g, col.b, 255)
	// 		x := obj.x >> pixelshift
	// 		y := obj.y >> pixelshift
	// 		renderer.FillRect(&sdl.Rect{x, y, 1, 1})
	// 	}
	// 	return // FIXME
	// }

	// Calculate the framebuffer offset of the center of the screen
	// if screen.MustLock() {
	// 	if err := screen.Lock(); err != nil {
	// 		die("Couldn't lock screen: %v", err)
	// 	}
	// }
	// bpp := screen.Format.BytesPerPixel
	CLIP(&obj.x, SW<<pixelshift)
	CLIP(&obj.y, SH<<pixelshift)
	x := obj.x / (1 << pixelshift)
	y := obj.y / (1 << pixelshift)
	xpitch := 1
	ypitch := screen.Stride
	screentop := 0
	screensize := screen.Rect.Dy() * ypitch
	screenbot := screensize
	pscreen0 := y * ypitch
	pscreen0 += x * xpitch

	if obj.typ != dust {
		// Now look at the sprite
		image := sprite[obj.typ+obj.state+(obj.phase&0xFF)]
		psprite0 := int(0)

		// XX--  0123
		// XX--  4567

		//        *pscreen0 = bullet_colour;
		for yc := image.Rect.Dy() - 1; yc >= 0; yc-- {
			psprite := psprite0

			//            if (pscreen0 < screentop)
			//                pscreen0 += screensize;
			pscreen := pscreen0

			for xc := image.Rect.Dx() - 1; xc >= 0; xc-- {
				if pscreen >= screenbot {
					pscreen -= screensize
				}
				if CHECK_PLOT {
					if pscreen < screentop || pscreen >= screenbot {
						die("Attempt to plot sprite out of screen at %p start %p end %p\n", pscreen, screentop, screenbot)
					}
				}
				if image.Pix[psprite] != 0 {
					if screen.Pix[pscreen] == bullet_colour {
						hit = pscreen
					} else if screen.Pix[pscreen] != 0 {
						collision = pscreen
					}
					if obj.typ == bullet {
						screen.Pix[pscreen] = bullet_colour
					} else {
						screen.Pix[pscreen] = image.Pix[psprite]
					}
				}
				pscreen++
				psprite++
			}
			pscreen0 += screen.Stride
			if pscreen0 >= screenbot {
				pscreen0 -= screensize
			}
			psprite0 += image.Stride
		}

		// work out where the hit or collision was
		if hit != 0 {
			offset := hit - screentop
			hit_y = offset / screen.Stride
			hit_x = (offset - hit_y*screen.Stride) << pixelshift
			hit_y <<= pixelshift
			// debugf("hit at %p (%i,%i) start %p end %p\n", hit, hit_x/(1<<pixelshift), hit_y/(1<<pixelshift), screentop, screenbot);
		}
		if collision != 0 {
			offset := collision - screentop
			collision_y = offset / screen.Stride
			collision_x = (offset - collision_y*screen.Stride) << pixelshift
			collision_y <<= pixelshift
			// debugf("collision at %p (%i,%i) start %p end %p\n", collision, collision_x, collision_y, screentop, screenbot);
		}
	} else {
		fade := (obj.life - time_now) >> 2
		fade = 15 - fade
		if fade > 15 {
			fade = 15
		}
		if fade < 0 {
			fade = 0
		}
		if CHECK_PLOT {
			if pscreen0 < screentop || pscreen0 >= screenbot {
				die("Attempt to plot dust off screen at %p start %p end %p", pscreen0, screentop, screenbot)
			}
		}
		screen.Pix[pscreen0] = fade_colour[obj.state][fade]
	}

	// // Update the display
	// if screen.MustLock() {
	// 	screen.Unlock()
	// }
}

// This plots all the objects currently in the objects data structure
// and does the collision detection
func plotobjects() {
	// first plot the bullets, and update everything
	for i := 0; i < maxobjects; i++ {
		obj := &object[i]
		if obj.typ != 0 {
			update(obj)
			if obj.typ == bullet {
				plot(obj)
			}
		}
	}

	// now plot the roids
	for i := 0; i < maxobjects; i++ {
		obj := &object[i]
		if obj.typ == roid {
			plot(obj)
			if hit != 0 {
				blowroid(obj, hit_x, hit_y)
			}
		}
	}

	// now plot the ship
	if object[0].typ == ship {
		plot(&object[0])

		if !ghostship && collision != 0 {
			blowship(collision_x, collision_y)
		}
	}

	// now plot the bullets again
	for i := 0; i < maxobjects; i++ {
		obj := &object[i]
		if obj.typ == bullet {
			plot(obj)
			if collision != 0 {
				obj.typ = 0
				roidexplosion(obj, collision_x, collision_y)
			}
		}
	}

	// now plot the pretty objects (dust, score etc)
	for i := 0; i < maxobjects; i++ {
		obj := &object[i]
		if obj.typ >= pretty {
			plot(obj)
		}
	}
}

// Read the keys
func readkeys() {
	event := sdl.PollEvent()
	if event != nil && event.GetType() == sdl.QUIT {
		fmt.Printf("SDL quit received - bye\n")
		os.Exit(0)
	}

	if time_now-key_time < 0 {
		return
	} // don't read keys too often
	key_time += 5

	pressed := sdl.GetKeyboardState()

	// if ship is dead, no need to read action keys
	if object[0].typ != 0 {
		if pressed[sdl.SCANCODE_Z] != 0 {
			object[0].state = (ship_direction() + 1) & 0xF
		}
		if pressed[sdl.SCANCODE_X] != 0 {
			object[0].state = (ship_direction() - 1) & 0xF
		}
		if pressed[sdl.SCANCODE_RSHIFT] != 0 || pressed[sdl.SCANCODE_LSHIFT] != 0 {
			c := cos_table[ship_direction()]
			s := sin_table[ship_direction()]
			object[0].dx += c / (1 << (18 - pixelshift))
			object[0].dy += s / (1 << (18 - pixelshift))
		}
		if !ghostship && pressed[sdl.SCANCODE_RETURN] != 0 {
			fire()
		}
	}

	spacepressed = pressed[sdl.SCANCODE_SPACE] != 0

	if pressed[sdl.SCANCODE_ESCAPE] != 0 {
		fmt.Printf("Escape pressed - bye\n")
		os.Exit(0)
	}
}

// Slow the ship down due to friction - unphysical but makes the game
// playable
func shipfriction() {
	dx := object[0].dx
	dy := object[0].dy

	dx -= dx / (1 << logshipfriction)
	if dx < 0 {
		dx += 1
	}
	if dx > 0 {
		dx -= 1
	}

	dy -= dy / (1 << logshipfriction)
	if dy < 0 {
		dy += 1
	}
	if dy > 0 {
		dy -= 1
	}

	object[0].dx = dx
	object[0].dy = dy
}

// This should be called after modifying the screen
func startupdate() {
	start_update_time = get_ticks()

	//bank = 3 - bank
	// change logical bank
	// update screenstart

	// Clear the screen
	//screen.FillRect(nil, 0)
	renderer.SetDrawColor(0, 0, 0, 255)
	renderer.Clear()

	// Clear the screen to black
	for i := range screen.Pix {
		screen.Pix[i] = 0
	}
}

// This should be called after modifying the screen
func endupdate() {
	// wait for Vsync
	// change physical bank
	// writeImage(screen, "/tmp/screen.png")

	// super simple image conversion
	for y := 0; y < screen.Rect.Dy(); y += 1 {
		for x := 0; x < screen.Rect.Dx(); x += 1 {
			c := screen.ColorIndexAt(x, y)
			if c != 0 {
				if int(c) >= len(palette) {
					debugf("color out of range %d/%d", c, len(palette))
					c = uint8(len(palette) - 1)
				}
				cc := palette[c].(color.RGBA)
				renderer.SetDrawColor(cc.R, cc.G, cc.B, cc.A)
				renderer.DrawPoint(int32(x), int32(y))
			}
		}
	}

	// var rect sdl.Rect
	// rect.X = 0
	// rect.Y = 0
	// rect.W = int32(screen.Rect.Dx())
	// rect.H = int32(screen.Rect.Dy())

	// texture, err := renderer.CreateTextureFromSurface(s)
	// if err != nil {
	// 	die("Create texture from surface error: %v", err)
	// }
	// defer texture.Destroy()

	// renderer.Copy(texture, nil, &rect)

	// Update all the changed bits
	// sdl.UpdateRects(screen, 1, &rect)
	// FIXME
	// if err := screen.Flip(); err != nil {
	// 	die("Problem with sdl.Flip: %v", err)
	// }
	renderer.Present()

	// Make sure we aren't showing frames too quickly
	plot_time := get_ticks() - start_update_time
	pause_time := MIN_MS_PER_FRAME - plot_time
	if pause_time > 0 {
		//sdl.Delay(pause_time)
		time.Sleep(time.Millisecond * time.Duration(pause_time)) // FIXME
	}
}

// This resets the bank switching

func resetbanks() {
	// bank = 2
	startupdate()
	endupdate()
}

// This plots a single frame of the game onto the screen
func plotframe() {
	new_time := read_time()
	dtime = new_time - time_now
	time_now = new_time

	makedigits(score, sscore)

	if score > hscore {
		hscore = score
	}

	makedigits(hscore, shscore)
	makedigits(plot_time, sframems)

	startupdate()
	plotobjects()
	endupdate()

	readkeys()
	shipfriction()
}

// This plots the game objects for r0 centi-seconds before returning control
func gamepause(pause int) {
	end_time := time_now + pause

	for time_now-end_time < 0 {
		plotframe()
	}
}

// This is run when the game is over
func gameover() {
	obj := findspacenzero()

	obj.x = (SW - sprite[gameoverword].Rect.Dx()) << (pixelshift - 1)
	obj.y = (SH - sprite[gameoverword].Rect.Dy()) << (pixelshift - 1)
	obj.typ = gameoverword

	gamepause(500)

	obj.typ = 0
}

// This is run when a life has been lost, and it is necessary to
// restart the game, or the game is over
func lifelost() {
	slives[lives].typ = 0 // cancel the last life used

	lives--
	if lives < 0 {
		return
	}

	gamepause(300)
	ghostship = true

	// reset the ship
	object[0].typ = ship
	object[0].dx = 0
	object[0].dy = 0
	object[0].x = (SW / 2) << pixelshift
	object[0].y = (SH / 2) << pixelshift

	obj := findspacenzero()
	obj.typ = numbers

	// Count down the time to go
	end_time := time_now + 5*64 - 4
	for time_now-end_time < 0 {
		obj.x = object[0].x - (8 << pixelshift)
		obj.y = object[0].y - (8 << pixelshift)
		obj.state = (((end_time - time_now) >> 6) + 1) & 7
		plotframe()
	}

	obj.typ = 0
	ghostship = false
}

// This shows the instructions on the screen, and returns control when
// space has been pressed
func showinstructions() {
	obj := findspacenzero()
	obj.x = (SW - sprite[instructionsword].Rect.Dx()) << (pixelshift - 1)
	obj.y = (SH - sprite[instructionsword].Rect.Dy()) << (pixelshift - 1)
	obj.typ = instructionsword

	ghostship = true

	for {
		plotframe()
		if spacepressed {
			break
		}
	}

	obj.typ = 0
	ghostship = false
}

// This generates a new roid with random position, not too close to the
// centre
func makeroid() {
	obj := findspace()

	nroids++

	var x, y int
	for {
		x = rand.Int() % SW // x = 0..SW-1
		y = rand.Int() % SH // y = 0..SH-1

		// distance from centre, to centre of roid squared
		if (x-(SW/2)-16)*(x-(SW/2)-16)+(y-(SH/2)-16)*(y-(SH/2)-16) > 20480 {
			break
		}
	}

	obj.x = x << pixelshift
	obj.y = y << pixelshift
	obj.dx = (rand.Int() & 0xFF) - 0x80
	obj.dy = (rand.Int() & 0xFF) - 0x80

	obj.state = 0
	obj.life = 0
	obj.phase = 0
	obj.lives = 0
	obj.typ = roid
}

func main() {
	defer screen_initialise()()
	load_sprites()
	make_screen()
	init_vars()
	resetbanks()
	time_now = read_time()

	// new game
	for {
		setupobjects()
		makeroid()
		showinstructions()
		setupobjects()

		// new level
		for lives >= 0 {

			level++
			if level != 0 {
				gamepause(300)
			}

			makedigits(level, slevel)

			for i := level + 1; i > 0; i-- {
				makeroid()
			}

			// main loop
			for lives >= 0 && nroids != 0 {
				if object[0].typ != ship {
					lifelost()
				}
				plotframe()
			}
		}
		gameover()
	}
}
