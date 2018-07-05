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

// Create bindata.go to embed the images
//go:generate go-bindata -nometadata -nocompress images

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"math/rand"
	"os"
	"time"
)

const (
	SW                  = 320
	SH                  = 256
	check_plot          = true
	pixelshift          = 8
	logshipfriction     = 8
	maxobjects          = 4096 // must be a power of two
	maxsprites          = 64
	min_ms_per_frame    = 20
	fade_colours        = 256
	log_cos_table_scale = 16
	cos_table_scale     = (1 << log_cos_table_scale)
	scale               = 3 // multiply the screen by this much
)

// structures
type object struct {
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

// Clear the object
func (obj *object) clear() {
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

// Game state
type Game struct {
	screen       *image.Paletted
	palette      color.Palette
	foundPalette map[rgba]uint8
	sprite       [maxsprites]*image.Paletted
	objects      [maxobjects]object
	funcStack    []func()

	// Keys
	keyZ      bool
	keyX      bool
	keyShift  bool
	keyReturn bool
	keySpace  bool

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
	bullet_colour uint8

	slives   []*object
	sscore   []*object
	shscore  []*object
	slevel   []*object
	sframems []*object

	start_update_time int
	plot_time         int // time it takes to plot a frame in ms

	// sprite numbers

	spritepointer, ship, roid, roid_endsplit, roid_endsmall        int
	roid_attacker, roid_shooter, roid_weapon, bullet, pretty, dust int
	levelword, scoreword, highscoreword, johnroidsword, showlife   int
	numbers, instructionsword, gameoverword                        int
}

func New() *Game {
	g := new(Game)
	g.foundPalette = make(map[rgba]uint8)
	g.slives = make([]*object, 5)
	g.sscore = make([]*object, 6)
	g.shscore = make([]*object, 6)
	g.slevel = make([]*object, 2)
	g.sframems = make([]*object, 3)

	g.load_sprites()
	g.make_screen()
	g.init_vars()
	g.resetbanks()
	g.time_now = read_time()
	g.push(g.newGame)
	return g
}

// Push functions on the execution stack.
//
// If more than one function is supplied, eg g.push(fn1, fn2) these
// are executed in the order fn(1), fn(2)
func (g *Game) push(fns ...func()) {
	for i := range fns {
		g.funcStack = append(g.funcStack, fns[len(fns)-1-i])
	}
}

// Pop a single function from the stack, returns nil if not found
func (g *Game) pop() (fn func()) {
	if len(g.funcStack) == 0 {
		return nil
	}
	fn, g.funcStack = g.funcStack[len(g.funcStack)-1], g.funcStack[:len(g.funcStack)-1]
	return fn
}

// Write some debug stuff
func debugf(format string, a ...interface{}) {
	fmt.Printf(format+"\n", a...)
}

// Blow up with a fatal error
func die(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
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

// This finds a space in the objects, returning the address of that
// space in r0, or 0 if there was no space
func (g *Game) findspace() *object {
	o := rand.Int() & (maxobjects - 1)
	if o == 0 { // don't use the ship (first) slot
		o++
	}

	for i := maxobjects - 10; i > 0; i-- { // try maxobjects-10 slots
		if g.objects[o].typ == 0 {
			return &g.objects[o]
		}
		o++
		if o >= maxobjects {
			o = 1
		}
	}

	debugf("Out of objects\n")
	return nil
}

// This finds a space in the objects, returning the address of that
// space in r0, or 0 if there was no space.
// It also zeros all the entries in that object
func (g *Game) findspacenzero() *object {
	obj := g.findspace()
	if obj == nil {
		return nil
	}

	obj.clear()

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
func (g *Game) reorderPalette(img *image.Paletted) {
	var pixmap [256]uint8
	// Read the colors out of the palette
	for i, c := range img.Palette {
		pixmap[i] = g.foundPalette[newRGBA(c)]
	}
	// swizzle the image data
	for i := range img.Pix {
		img.Pix[i] = pixmap[img.Pix[i]]
	}
	// replace the pallete with the standard one
	img.Palette = g.palette
}

// Make palette from the loaded values then make the screen
func (g *Game) make_screen() {
	// black must be the first entry
	g.palette = append(g.palette, color.RGBA{0, 0, 0, 255})
	black := rgba{0, 0, 0, 255}
	g.foundPalette[black] = 0
	for c := range g.foundPalette {
		if c == black {
			continue
		}
		R, B, G, A := c.r, c.b, c.g, c.a
		g.palette = append(g.palette, color.RGBA{R, G, B, A})
		g.foundPalette[c] = uint8(len(g.palette) - 1)
	}
	debugf("Found %d colors", len(g.palette))
	if len(g.palette) > 256 {
		die("too many colors")
	}

	// rewrite the palettes of all the sprites
	for i := 0; i < g.spritepointer; i++ {
		g.reorderPalette(g.sprite[i])
	}

	// work out what the bullet_colour is
	b := g.sprite[g.bullet]
	for x := 0; x < b.Rect.Dx(); x++ {
		for y := 0; y < b.Rect.Dy(); y++ {
			p := b.ColorIndexAt(x, y)
			if p != 0 {
				g.bullet_colour = p
			}
		}
	}

	// Now we've loaded all the sprites we can make the screen
	g.screen = image.NewPaletted(image.Rect(0, 0, SW, SH), g.palette)

	// make the fade colours
	for i, col := range g.palette {
		R, G, B, A := col.RGBA()
		R >>= 8
		G >>= 8
		B >>= 8
		A >>= 8
		// debugf("r,g,b = %d,%d,%d", r, g, b)
		for j := uint32(0); j < 16; j++ {
			k := 16 - j
			R, G, B := (R*k)/16, (G*k)/16, (B*k)/16
			paletteCol := g.palette.Convert(color.RGBA{uint8(R), uint8(G), uint8(B), 255})
			pixel := g.foundPalette[newRGBA(paletteCol)]
			// debugf("fade_colour[%d][%d]=%d (%d,%d,%d)", i, j, pixel, (r*k)/16, (g*k)/16, (b*k)/16)
			g.fade_colour[i][j] = pixel
		}
	}
}

// Load sprite
func (g *Game) load_sprite(name string) int {
	path := fmt.Sprintf("images/%s.png", name)
	// debugf("Loading sprite '%s' from '%s'", name, path)
	// fd, err := os.Open(path)
	// if err != nil {
	// 	die("Failed to open image %s: %v", path, err)
	// }
	// defer fd.Close()

	data := MustAsset(path)

	goImage, _, err := image.Decode(bytes.NewReader(data))
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
	g.sprite[g.spritepointer] = palettedImage

	// Read the colors out of the palette
	for _, c := range palettedImage.Palette {
		g.foundPalette[newRGBA(c)] = 0
	}

	if g.spritepointer >= maxsprites {
		die("Too many sprites")
	}

	n := g.spritepointer
	g.spritepointer++
	return n
}

// Load sprites
func (g *Game) load_sprites() {
	g.spritepointer = 0

	g.load_sprite("0")

	g.ship = g.spritepointer
	for i := 0; i <= 15; i++ {
		ship_name := fmt.Sprintf("ship%d", i)
		g.load_sprite(ship_name)
	}

	// roids split into n -> ( 2n+1, 2n+2 )
	g.roid = g.spritepointer
	g.load_sprite("0")   // 0 -> 1,2
	g.load_sprite("00")  // 1 -> 3,4
	g.load_sprite("01")  // 2 -> 5,6
	g.load_sprite("000") // 3 -> 7,8
	g.load_sprite("001") // 4 -> 9,10
	g.load_sprite("010") // 5 -> 11,12
	g.load_sprite("011") // 6 -> 13,14
	g.roid_endsplit = g.spritepointer - 1 - g.roid
	g.load_sprite("0000") // 7
	g.load_sprite("0001") // 8
	g.load_sprite("0010") // 9
	g.load_sprite("0011") // 10
	g.load_sprite("0100") // 11
	g.load_sprite("0101") // 12
	g.load_sprite("0110") // 13
	g.load_sprite("0111") // 14
	g.roid_endsmall = g.spritepointer - 1 - g.roid

	g.roid_attacker = g.spritepointer - g.roid
	g.load_sprite("attacker")

	g.roid_shooter = g.spritepointer - g.roid
	g.load_sprite("shooter")

	g.roid_weapon = g.spritepointer - g.roid
	g.load_sprite("weapon0")
	g.load_sprite("weapon1")
	g.load_sprite("weapon2")
	g.load_sprite("weapon3")

	g.bullet = g.spritepointer
	g.load_sprite("missile")

	// all sprites after here are just for effect

	g.pretty = g.spritepointer

	g.dust = g.spritepointer
	g.load_sprite("dust")

	g.levelword = g.spritepointer
	g.load_sprite("level")

	g.scoreword = g.spritepointer
	g.load_sprite("score")

	g.highscoreword = g.spritepointer
	g.load_sprite("highscore")

	g.johnroidsword = g.spritepointer
	g.load_sprite("johnroids")

	g.showlife = g.spritepointer
	g.load_sprite("ship4")

	g.numbers = g.spritepointer
	g.load_sprite("n0")
	g.load_sprite("n1")
	g.load_sprite("n2")
	g.load_sprite("n3")
	g.load_sprite("n4")
	g.load_sprite("n5")
	g.load_sprite("n6")
	g.load_sprite("n7")
	g.load_sprite("n8")
	g.load_sprite("n9")

	g.instructionsword = g.spritepointer
	g.load_sprite("instrs")

	g.gameoverword = g.spritepointer
	g.load_sprite("gameover")
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

func (g *Game) init_vars() {
	rand.Seed(time.Now().UTC().UnixNano())

	for i := 0; i < 16; i++ {
		g.cos_table[i] = int(cos_table_scale*math.Cos(float64(i)/16*2*math.Pi) + 0.5)
		g.sin_table[i] = int(cos_table_scale*math.Sin(-float64(i)/16*2*math.Pi) + 0.5)
	}

	for i := 0; i < 256; i++ {
		g.cos_table2[i] = int(cos_table_scale*math.Cos(float64(i)/256*2*math.Pi) + 0.5)
		g.sin_table2[i] = int(cos_table_scale*math.Sin(-float64(i)/256*2*math.Pi) + 0.5)
	}
}

// This sets up all the objects needed at the start of a game
// This includes the score, highscore, and the lives.
// It removes any previous objects
func (g *Game) setupobjects() {
	for i := 0; i < maxobjects; i++ {
		g.objects[i].clear()
	}

	g.lives = 3

	g.ghostship = false
	g.score = 0
	g.level = 0
	g.nroids = 0

	g.objects[0].typ = g.ship
	g.objects[0].state = 0
	g.objects[0].phase = 0
	g.objects[0].lives = 0
	g.objects[0].life = 0
	g.objects[0].dx = 0
	g.objects[0].dy = 0
	g.objects[0].x = (SW / 2) << pixelshift
	g.objects[0].y = (SH / 2) << pixelshift

	obj := g.findspacenzero()
	obj.typ = g.scoreword

	x := int(48) << pixelshift
	for i := 0; i < 6; i++ {
		obj = g.findspacenzero()
		g.sscore[i] = obj
		obj.typ = g.numbers
		obj.x = x
		x += 8 << pixelshift
	}

	obj = g.findspacenzero()
	obj.typ = g.levelword
	obj.x = (SW/2 - 46) << pixelshift // 114<<pixelshift;

	g.slevel[0] = g.findspacenzero()
	g.slevel[0].typ = g.numbers
	g.slevel[0].x = (SW/2 - 2) << pixelshift // 158<<pixelshift;

	g.slevel[1] = g.findspacenzero()
	g.slevel[1].typ = g.numbers
	g.slevel[1].x = (SW/2 + 6) << pixelshift // 166<<pixelshift;

	x = 0 << pixelshift
	for i := 0; i < 5; i++ { // can have up to 5 lives
		obj = g.findspacenzero()
		g.slives[i] = obj
		if i > 3 {
			obj.typ = 0
		} else {
			obj.typ = g.showlife // 3 lives to start with
		}

		obj.y = (SH - 12) << pixelshift
		obj.x = x
		x += 8 << pixelshift
	}

	obj = g.findspacenzero()
	obj.typ = g.highscoreword
	obj.x = (SW - 132) << pixelshift

	x = (SW - 48) << pixelshift // 272
	for i := 0; i < 6; i++ {
		obj = g.findspacenzero()
		g.shscore[i] = obj
		obj.typ = g.numbers
		obj.x = x
		x += 8 << pixelshift
	}

	x = (SW - 24) << pixelshift
	for i := 0; i < 3; i++ {
		obj = g.findspacenzero()
		g.sframems[i] = obj
		obj.typ = g.numbers
		obj.y = (SH - 12) << pixelshift
		obj.x = x
		x += 8 << pixelshift
	}
}

// This converts the number in r0, into r1 decimal digits, putting
// the results into the objects pointed to by the array of pointers pointed
// to by r2
func (g *Game) makedigits(n int, output []*object) {
	digitbuffer := fmt.Sprintf("%0*d", len(output), n)
	for i, c := range digitbuffer {
		output[i].state = int(c - '0')
	}
}

// This fires a bullet from a roid, pointed to in r0
func (g *Game) roidfire(parent *object) {
	weapon := g.findspace()

	weapon.typ = g.roid
	weapon.state = g.roid_weapon
	weapon.life = 0
	weapon.lives = 0
	weapon.phase = (rand.Int() & 3) | (0x03 << 8)
	weapon.x = parent.x
	weapon.y = parent.y

	// calculate a trajectory to fire at the ship
	dx := g.objects[0].x - weapon.x
	dy := g.objects[0].y - weapon.y
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

	g.nroids++
}

// return the current ship direction
func (g *Game) ship_direction() int {
	return g.objects[0].state
}

// This fires a bullet from the ship
func (g *Game) fire() {
	obj := g.findspace()

	g.score--
	if g.score < 0 {
		g.score = 0
	}

	obj.typ = g.bullet
	obj.state = 0
	obj.phase = 0
	obj.lives = 0

	obj.life = g.time_now + 200

	x := g.objects[0].x
	y := g.objects[0].y
	dx := g.objects[0].dx
	dy := g.objects[0].dy

	c := g.cos_table[g.ship_direction()]
	s := g.sin_table[g.ship_direction()]

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
func (g *Game) blowroid(r *object, x int, y int) {
	g.score += r.state * 10

	if r.state > g.roid_endsplit {
		// don't split this roid into two
		if r.state > g.roid_endsmall || (rand.Int()&0xFF) > (0x100/10) {
			r.typ = 0 // destroy roid
			g.nroids--
		} else {
			// make an attacking roid if small roid and chance
			r.state = g.roid_attacker
			r.x = x - (8 << pixelshift)
			r.y = y - (8 << pixelshift)
			// debugf("roid_attacker at (%g,%g)\n", (double)x/(1<<pixelshift), (double)y/(1<<pixelshift));
		}
	} else {
		if r.state == 0 && (rand.Int()&0xFF) < (0x100/2) {
			// make a shooting roid if big roid and chance
			r.state = g.roid_shooter
			r.x += ((32 - 24) / 2) << pixelshift
			r.y += ((32 - 24) / 2) << pixelshift
			g.roidfire(r) // make it fire
		} else {
			s := g.findspace()

			// split the roid into two
			r.state = 2*r.state + 1
			s.typ = g.roid
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

			g.nroids++
		}
	}
}

// This explodes an object into its component pixels with a blast centre
// of (x,y).  Use (-1,-1) to use the centre of the object.
func (g *Game) explode_object(obj *object, x int, y int, particles int, mean_speed int, speed_mask int, life_mask int) {
	image := g.sprite[obj.typ+obj.state+(obj.phase&0xFF)]
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
				p := g.findspace()
				p.typ = g.dust
				p.state = int(colour)
				p.life = g.time_now + (rand.Int() & life_mask)
				p.x = obj.x + (i << pixelshift) + (rand.Int() & 0x1F) - 0x10
				p.y = obj.y + (j << pixelshift) + (rand.Int() & 0x1F) - 0x10
				speed := mean_speed
				if speed_mask != 0 {
					speed -= (rand.Int() & speed_mask)
				}
				//p.dx = dx + ( cos_table2[angle >> 8] * speed ) / cos_table_scale;
				//p.dy = dy + ( sin_table2[angle >> 8] * speed ) / cos_table_scale;
				//p.dx = dx + (rand.Int() & 0xFF) - 0x80;
				//p.dy = dy + (rand.Int() & 0xFF) - 0x80;
				p.dx = obj.dx + (i << (pixelshift - 3)) - (image.Rect.Dx() << (pixelshift - 4))
				p.dy = obj.dy + (j << (pixelshift - 3)) - (image.Rect.Dy() << (pixelshift - 4))
				angle += dangle
			}
		}
	}
}

// This makes an explosion at the object pointed to by r0
// with r1 particles, of colour_row r2, or random if r2<0
// r3 is used to mask centi-seconds of life
func (g *Game) explosion(obj *object, x int, y int, particles int, mean_speed int, speed_mask int, colour_row int, life_mask int) {
	image := g.sprite[obj.typ+obj.state+(obj.phase&0xFF)]
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
		p := g.findspace()
		p.typ = g.dust
		p.state = colour_row
		if colour_row < 0 {
			p.state = (rand.Int() % fade_colours)
		}
		// p.life = time_now + (rand.Int() & life_mask);
		p.life = g.time_now + life_mask
		p.x = x
		p.y = y
		angle = rand.Int() & 0xFFFF
		// angle += dangle;
		speed := mean_speed
		if speed_mask != 0 {
			speed -= (rand.Int() & speed_mask)
		}
		p.dx = dx + (g.cos_table2[angle>>8]*speed)/cos_table_scale
		p.dy = dy + (g.sin_table2[angle>>8]*speed)/cos_table_scale
		// p.dx = dx + (rand.Int() & 0xFF) - 0x80;
		// p.dy = dy + (rand.Int() & 0xFF) - 0x80;
	}
}

// This blows up the ship
func (g *Game) blowship(x int, y int) {
	// sound_blowship()
	g.explode_object(&g.objects[0], x, y, 64, 0x100, 0x7F, 0x1FF)
	g.explosion(&g.objects[0], x, y, 64, 0x80, 0x1F, -1, 0x1FF)
	g.objects[0].typ = 0
}

// This explodes a bullet that has hit, or is time expired
func (g *Game) bulletexplosion(obj *object) {
	g.explode_object(obj, -1, -1, 8, 0x20, 0x00, 0x7F)
	//    explosion(obj, 8, 0x20, 0x00, bullet_colour, 0x7F);
	// sound_bulletexplosion();
}

// This explodes a roid
func (g *Game) roidexplosion(obj *object, x int, y int) {
	//    explode_object(obj, x, y, 32, 0x80, 0x7F, 0xFF);
	g.explosion(obj, x, y, 32, 0x80, 0x7F, rand.Int()%fade_colours, 0xFF)
	// sound_roidexplosion();
}

// clip *z into the range 0 <= *z < w
func clip(z *int, w int) {
	if *z < 0 {
		*z += w
	}
	if *z >= w {
		*z -= w
	}
}

// This updates the co-ordinates of the object in r0 co-ordinates
func (g *Game) update(obj *object) {
	sy := obj.y
	sx := obj.x

	sx = g.dtime*obj.dx + sx // scale for time
	clip(&sx, SW<<pixelshift)
	obj.x = sx // x+=dx

	sy = g.dtime*obj.dy + sy // scale for time
	clip(&sy, SH<<pixelshift)
	obj.y = sy // y+=dy

	// if object is mortal and over age limit, kill it
	if obj.life != 0 && obj.life-g.time_now <= 0 {
		// debugf("Killing object %i %i %i\n", obj.life, time_now, obj.life - time_now);
		if obj.typ == g.bullet {
			g.bulletexplosion(obj)
		}
		obj.typ = 0
		// return; FIXME ;;;; ???
	}

	// increment object phase
	obj.phase = (obj.phase & 0xFF00) | ((obj.phase + 1) & (obj.phase >> 8))

	// if ship is dead, don't do the below
	if g.objects[0].typ != g.ship {
		return
	}

	// These deal with attacking the ship

	// Drive the attacker in decaying orbits around the ship
	if obj.typ == g.roid && obj.state == g.roid_attacker {
		dx := obj.dx
		dy := obj.dy

		dx -= (dx / (1 << 10))
		if g.objects[0].x > obj.x {
			dx += 4
		}
		if g.objects[0].x < obj.x {
			dx -= 4
		}
		obj.dx = dx

		dy -= (dy / (1 << 10))
		if g.objects[0].y > obj.y {
			dy += 4
		}
		if g.objects[0].y < obj.y {
			dy -= 4
		}
		obj.dy = dy
	}

	// make sure the shooter shoots now and again
	if obj.typ == g.roid && obj.state == g.roid_shooter &&
		!g.ghostship && (rand.Int()&0xFF) < (0xFF/20) {
		g.roidfire(obj)
	}
}

// Plot a pixel of a given colour directly to the screen
func (g *Game) plot_pixel(X int, Y int, pixel uint8) {
	if X < 0 || X >= g.screen.Rect.Dx() || Y < 0 || Y >= g.screen.Rect.Dy() {
		debugf("Out of bounds pixel %i,%i", X, Y)
		return
	}

	g.screen.SetColorIndex(X, Y, pixel)
}

// this plots the object whose address is in r0
func (g *Game) plot(obj *object) {
	g.hit = 0
	g.collision = 0

	clip(&obj.x, SW<<pixelshift)
	clip(&obj.y, SH<<pixelshift)
	x := obj.x / (1 << pixelshift)
	y := obj.y / (1 << pixelshift)
	xpitch := 1
	ypitch := g.screen.Stride
	screentop := 0
	screensize := g.screen.Rect.Dy() * ypitch
	screenbot := screensize
	pscreen0 := y * ypitch
	pscreen0 += x * xpitch

	if obj.typ != g.dust {
		// Now look at the sprite
		image := g.sprite[obj.typ+obj.state+(obj.phase&0xFF)]
		psprite0 := int(0)

		for yc := image.Rect.Dy() - 1; yc >= 0; yc-- {
			psprite := psprite0
			pscreen := pscreen0

			for xc := image.Rect.Dx() - 1; xc >= 0; xc-- {
				if pscreen >= screenbot {
					pscreen -= screensize
				}
				if check_plot {
					if pscreen < screentop || pscreen >= screenbot {
						die("Attempt to plot sprite out of screen at %p start %p end %p\n", pscreen, screentop, screenbot)
					}
				}
				if image.Pix[psprite] != 0 {
					if g.screen.Pix[pscreen] == g.bullet_colour {
						g.hit = pscreen
					} else if g.screen.Pix[pscreen] != 0 {
						g.collision = pscreen
					}
					if obj.typ == g.bullet {
						g.screen.Pix[pscreen] = g.bullet_colour
					} else {
						g.screen.Pix[pscreen] = image.Pix[psprite]
					}
				}
				pscreen++
				psprite++
			}
			pscreen0 += g.screen.Stride
			if pscreen0 >= screenbot {
				pscreen0 -= screensize
			}
			psprite0 += image.Stride
		}

		// work out where the hit or collision was
		if g.hit != 0 {
			offset := g.hit - screentop
			g.hit_y = offset / g.screen.Stride
			g.hit_x = (offset - g.hit_y*g.screen.Stride) << pixelshift
			g.hit_y <<= pixelshift
			// debugf("hit at %p (%i,%i) start %p end %p\n", hit, hit_x/(1<<pixelshift), hit_y/(1<<pixelshift), screentop, screenbot);
		}
		if g.collision != 0 {
			offset := g.collision - screentop
			g.collision_y = offset / g.screen.Stride
			g.collision_x = (offset - g.collision_y*g.screen.Stride) << pixelshift
			g.collision_y <<= pixelshift
			// debugf("collision at %p (%i,%i) start %p end %p\n", collision, collision_x, collision_y, screentop, screenbot);
		}
	} else {
		fade := (obj.life - g.time_now) >> 2
		fade = 15 - fade
		if fade > 15 {
			fade = 15
		}
		if fade < 0 {
			fade = 0
		}
		if check_plot {
			if pscreen0 < screentop || pscreen0 >= screenbot {
				die("Attempt to plot dust off screen at %p start %p end %p", pscreen0, screentop, screenbot)
			}
		}
		g.screen.Pix[pscreen0] = g.fade_colour[obj.state][fade]
	}
}

// This plots all the objects currently in the objects data structure
// and does the collision detection
func (g *Game) plotobjects() {
	// first plot the bullets, and update everything
	for i := 0; i < maxobjects; i++ {
		obj := &g.objects[i]
		if obj.typ != 0 {
			g.update(obj)
			if obj.typ == g.bullet {
				g.plot(obj)
			}
		}
	}

	// now plot the roids
	for i := 0; i < maxobjects; i++ {
		obj := &g.objects[i]
		if obj.typ == g.roid {
			g.plot(obj)
			if g.hit != 0 {
				g.blowroid(obj, g.hit_x, g.hit_y)
			}
		}
	}

	// now plot the ship
	if g.objects[0].typ == g.ship {
		g.plot(&g.objects[0])

		if !g.ghostship && g.collision != 0 {
			g.blowship(g.collision_x, g.collision_y)
		}
	}

	// now plot the bullets again
	for i := 0; i < maxobjects; i++ {
		obj := &g.objects[i]
		if obj.typ == g.bullet {
			g.plot(obj)
			if g.collision != 0 {
				obj.typ = 0
				g.roidexplosion(obj, g.collision_x, g.collision_y)
			}
		}
	}

	// now plot the pretty objects (dust, score etc)
	for i := 0; i < maxobjects; i++ {
		obj := &g.objects[i]
		if obj.typ >= g.pretty {
			g.plot(obj)
		}
	}
}

// KeyCode defines keys to pass to Game.KeyEvent
type KeyCode byte

// Keys that need to be passed to Game.KeyEvent
const (
	KeyCodeZ KeyCode = iota
	KeyCodeX
	KeyCodeShift
	KeyCodeReturn
	KeyCodeSpace
)

// Key event should be used to deliver key presses to the game.
//
// key should be one of the define KeyCodes and pressed should be true
// for key depressed and false for key released
func (g *Game) KeyEvent(key KeyCode, pressed bool) {
	switch key {
	case KeyCodeZ:
		g.keyZ = pressed
	case KeyCodeX:
		g.keyX = pressed
	case KeyCodeShift:
		g.keyShift = pressed
	case KeyCodeReturn:
		g.keyReturn = pressed
	case KeyCodeSpace:
		g.keySpace = pressed
	default:
		debugf("Unknown key %v", key)
	}
}

// Read the keys
func (g *Game) readkeys() {
	// don't read keys too often
	if g.time_now-g.key_time < 0 {
		return
	}
	g.key_time += 5

	// if ship is dead, no need to read action keys
	if g.objects[0].typ != 0 {
		if g.keyZ {
			g.objects[0].state = (g.ship_direction() + 1) & 0xF
		}
		if g.keyX {
			g.objects[0].state = (g.ship_direction() - 1) & 0xF
		}
		if g.keyShift {
			c := g.cos_table[g.ship_direction()]
			s := g.sin_table[g.ship_direction()]
			g.objects[0].dx += c / (1 << (18 - pixelshift))
			g.objects[0].dy += s / (1 << (18 - pixelshift))
		}
		if !g.ghostship && g.keyReturn {
			g.fire()
		}
	}
}

// Slow the ship down due to friction - unphysical but makes the game
// playable
func (g *Game) shipfriction() {
	dx := g.objects[0].dx
	dy := g.objects[0].dy

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

	g.objects[0].dx = dx
	g.objects[0].dy = dy
}

// This should be called after modifying the screen
func (g *Game) startupdate() {
	g.start_update_time = get_ticks()

	// Clear the screen to black
	for i := range g.screen.Pix {
		g.screen.Pix[i] = 0
	}
}

// This should be called after modifying the screen
func (g *Game) endupdate() {
	// Make sure we aren't showing frames too quickly
	g.plot_time = get_ticks() - g.start_update_time
	pause_time := min_ms_per_frame - g.plot_time
	if pause_time > 0 {
		time.Sleep(time.Millisecond * time.Duration(pause_time)) // FIXME
	}
}

// This resets the bank switching
func (g *Game) resetbanks() {
	g.startupdate()
	g.endupdate()
}

// This plots a single frame of the game onto the screen
func (g *Game) plotframe() {
	new_time := read_time()
	g.dtime = new_time - g.time_now
	g.time_now = new_time

	g.makedigits(g.score, g.sscore)

	if g.score > g.hscore {
		g.hscore = g.score
	}

	g.makedigits(g.hscore, g.shscore)
	g.makedigits(g.plot_time, g.sframems)

	g.startupdate()
	g.plotobjects()
	g.endupdate()

	g.readkeys()
	g.shipfriction()
}

// This returns a function which will pause the game for pause centi-seconds
func (g *Game) gamepause(pause int) (fn func()) {
	end_time := g.time_now + pause
	return func() {
		if g.time_now-end_time < 0 {
			g.push(fn)
		}
	}
}

// This is run when the game is over
func (g *Game) gameover() {
	obj := g.findspacenzero()

	obj.x = (SW - g.sprite[g.gameoverword].Rect.Dx()) << (pixelshift - 1)
	obj.y = (SH - g.sprite[g.gameoverword].Rect.Dy()) << (pixelshift - 1)
	obj.typ = g.gameoverword

	g.push(
		g.gamepause(500),
		func() {
			obj.typ = 0
		},
	)
}

// This is run when a life has been lost, and it is necessary to
// restart the game, or the game is over
func (g *Game) lifelost() {
	g.slives[g.lives].typ = 0 // cancel the last life used

	g.lives--
	if g.lives < 0 {
		return
	}

	fn := func() {
		g.ghostship = true

		// reset the ship
		g.objects[0].typ = g.ship
		g.objects[0].dx = 0
		g.objects[0].dy = 0
		g.objects[0].x = (SW / 2) << pixelshift
		g.objects[0].y = (SH / 2) << pixelshift

		obj := g.findspacenzero()
		obj.typ = g.numbers

		// Count down the time to go
		end_time := g.time_now + 5*64 - 4
		var fn2 func()
		fn2 = func() {
			if g.time_now-end_time < 0 {
				obj.x = g.objects[0].x - (8 << pixelshift)
				obj.y = g.objects[0].y - (8 << pixelshift)
				obj.state = (((end_time - g.time_now) >> 6) + 1) & 7
				g.push(fn2)
			} else {
				obj.typ = 0
				g.ghostship = false
			}
		}
		fn2()
	}
	g.push(
		g.gamepause(300),
		fn,
	)
}

// This shows the instructions on the screen, and returns control when
// space has been pressed
func (g *Game) showinstructions() {
	obj := g.findspacenzero()
	obj.x = (SW - g.sprite[g.instructionsword].Rect.Dx()) << (pixelshift - 1)
	obj.y = (SH - g.sprite[g.instructionsword].Rect.Dy()) << (pixelshift - 1)
	obj.typ = g.instructionsword

	g.ghostship = true

	var fn func()
	fn = func() {
		if g.keySpace {
			obj.typ = 0
			g.ghostship = false
		} else {
			g.push(fn)
		}
	}
	fn()
}

// This generates a new roid with random position, not too close to the
// centre
func (g *Game) makeroid() {
	obj := g.findspace()

	g.nroids++

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
	obj.typ = g.roid
}

func (g *Game) newGame() {
	g.push(
		g.setupobjects,
		g.newLevel,
		g.gameover,
		g.newGame,
	)
	g.setupobjects()
	g.makeroid()
	g.showinstructions()
}

func (g *Game) newLevel() {
	if g.lives >= 0 {
		g.push(
			func() {
				g.level++
				if g.level != 0 {
					g.push(g.gamepause(300))
				}
			},
			func() {
				g.makedigits(g.level, g.slevel)
				for i := g.level + 1; i > 0; i-- {
					g.makeroid()
				}
			},
			g.gameLoop,
		)
	}
}

func (g *Game) gameLoop() {
	if g.lives >= 0 && g.nroids != 0 {
		g.push(g.gameLoop)
		if g.objects[0].typ != g.ship {
			g.lifelost()
		}
	} else {
		g.newLevel()
	}
}

// This plots a single frame of the game
func (g *Game) frame() *image.Paletted {
	fn := g.pop()
	if fn == nil {
		die("No functions on stack")
	}
	fn()
	g.plotframe()
	return g.screen
}
