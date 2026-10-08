// /
// / Package main selector screen: before an AI vs AI match starts this screen
// / lists every valid weights file next to the executable and lets the user
// / choose which network plays White and which plays Black.
package main

import (
	"image/color"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"chess/engine"
	"chess/nn"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
)

// /
// / <summary>
// /   Selector layout and palette constants.
// / </summary>
const (
	selListX    = 40
	selListW    = 560
	selRowH     = 38
	selRowGap   = 6
	selListY    = 196
	selStartW   = 240
	selStartH   = 48
	selStartGap = 16
)

// /
// / <summary>
// /   selColors holds the selector-specific colours.
// / </summary>
var (
	selRowBg     = color.RGBA{0x24, 0x22, 0x20, 0xFF}
	selRowCursor = color.RGBA{0x45, 0x43, 0x50, 0xFF}
	selWhiteTag  = color.RGBA{0xF2, 0xF2, 0xF2, 0xFF}
	selBlackTag  = color.RGBA{0x8A, 0x8A, 0x8A, 0xFF}
	selStartBg   = color.RGBA{0x2A, 0xC6, 0x5C, 0xFF}
	selStartOff  = color.RGBA{0x3A, 0x3A, 0x3A, 0xFF}
)

// /
// / <summary>
// /   listWeights finds every loadable network weights file next to the
// /   executable and in the working directory, returning absolute paths sorted
// /   by file name.
// / </summary>
// / <returns>Absolute paths of valid weights files.</returns>
func listWeights() []string {
	seen := map[string]bool{}
	var out []string
	dirs := []string{"."}
	if exe, err := os.Executable(); err == nil {
		dirs = append([]string{filepath.Dir(exe)}, dirs...)
	}
	for _, d := range dirs {
		entries, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			low := strings.ToLower(name)
			if !strings.HasSuffix(low, ".bin") {
				continue
			}
			if strings.HasPrefix(low, "dataset") || strings.HasPrefix(low, "relabel") {
				continue
			}
			p := filepath.Join(d, name)
			ap, err := filepath.Abs(p)
			if err != nil || seen[ap] {
				continue
			}
			if _, err := nn.Load(p); err != nil {
				continue
			}
			seen[ap] = true
			out = append(out, ap)
		}
	}
	sort.Slice(out, func(i, j int) bool { return baseName(out[i]) < baseName(out[j]) })
	return out
}

// /
// / <summary>
// /   findByName returns the index of the file whose base name matches, or -1.
// / </summary>
// / <param name="files">Candidate paths.</param>
// / <param name="name">Base file name to find.</param>
// / <returns>The index, or -1 when absent.</returns>
func findByName(files []string, name string) int {
	for i, p := range files {
		if baseName(p) == name {
			return i
		}
	}
	return -1
}

// /
// / <summary>
// /   selName renders the file name for a selection index.
// / </summary>
// / <param name="i">Selection index, or negative.</param>
// / <returns>The file name, or an em dash when unset.</returns>
func (g *Game) selName(i int) string {
	if i < 0 || i >= len(g.files) {
		return "—"
	}
	return baseName(g.files[i])
}

// /
// / <summary>
// /   selRowRect returns the pixel rectangle of a list row.
// / </summary>
// / <param name="i">Row index.</param>
// / <returns>Left, top, width and height in logical pixels.</returns>
func selRowRect(i int) (int, int, int, int) {
	y := selListY + i*(selRowH+selRowGap)
	return selListX, y, selListW, selRowH
}

// /
// / <summary>
// /   startRect returns the pixel rectangle of the start button.
// / </summary>
// / <returns>Left, top, width and height in logical pixels.</returns>
func (g *Game) startRect() (int, int, int, int) {
	y := selListY + len(g.files)*(selRowH+selRowGap) + selStartGap
	x := (screenW - selStartW) / 2
	return x, y, selStartW, selStartH
}

// /
// / <summary>
// /   rowHit maps a cursor position to a list row index.
// / </summary>
// / <param name="mx">Logical x cursor.</param>
// / <param name="my">Logical y cursor.</param>
// / <returns>The row index, or -1 when outside the list.</returns>
func (g *Game) rowHit(mx, my int) int {
	for i := range g.files {
		x, y, w, h := selRowRect(i)
		if mx >= x && mx < x+w && my >= y && my < y+h {
			return i
		}
	}
	return -1
}

// /
// / <summary>
// /   ready reports whether both sides have a network chosen.
// / </summary>
// / <returns>True when White and Black are both set.</returns>
func (g *Game) ready() bool {
	return g.selWhite >= 0 && g.selBlack >= 0
}

// /
// / <summary>
// /   assign binds a network to the side currently being chosen and advances
// /   the choice to the next side.
// / </summary>
// / <param name="i">Chosen list index.</param>
func (g *Game) assign(i int) {
	if i < 0 || i >= len(g.files) {
		return
	}
	if g.selStep == 0 {
		g.selWhite = i
		g.selStep = 1
		return
	}
	g.selBlack = i
	g.selStep = 0
}

// /
// / <summary>
// /   randomSelect picks two random networks, one per side, for a quick match.
// / </summary>
func (g *Game) randomSelect() {
	if len(g.files) == 0 {
		return
	}
	g.selWhite = rand.Intn(len(g.files))
	g.selBlack = rand.Intn(len(g.files))
	g.selStep = 0
}

// /
// / <summary>
// /   updateSelect handles keyboard and mouse input on the selection screen.
// / </summary>
func (g *Game) updateSelect() {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) || (ebiten.IsKeyPressed(ebiten.KeyControl) && inpututil.IsKeyJustPressed(ebiten.KeyQ)) {
		g.quit = true
		return
	}
	n := len(g.files)
	if n > 0 {
		if inpututil.IsKeyJustPressed(ebiten.KeyDown) {
			g.selCursor = (g.selCursor + 1) % n
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyUp) {
			g.selCursor = (g.selCursor - 1 + n) % n
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyR) {
			g.randomSelect()
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			if g.ready() {
				g.begin()
				return
			}
			g.assign(g.selCursor)
		}
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()
		if g.ready() {
			x, y, w, h := g.startRect()
			if mx >= x && mx < x+w && my >= y && my < y+h {
				g.begin()
				return
			}
		}
		if i := g.rowHit(mx, my); i >= 0 {
			g.assign(i)
		}
	}
}

// /
// / <summary>
// /   begin loads the chosen networks, installs them for each side and starts a
// /   fresh AI vs AI game.
// / </summary>
func (g *Game) begin() {
	if !g.ready() {
		return
	}
	wPath := g.files[g.selWhite]
	bPath := g.files[g.selBlack]
	whiteCfg = loadCfgPath(wPath)
	blackCfg = loadCfgPath(bPath)
	whiteName = baseName(wPath)
	blackName = baseName(bPath)

	g.choosing = false
	g.board = engine.NewStart()
	g.history = g.history[:0]
	g.keys = g.keys[:0]
	g.keys = append(g.keys, engine.PositionKey(g.board))
	g.selected = -1
	g.dests = nil
	g.anim = nil
	g.lastFrom, g.lastTo = -1, -1
	g.logs = g.logs[:0]
	g.randomizeOpening()
	g.resetLogs()
}

// /
// / <summary>
// /   drawSelect renders the network selection screen.
// / </summary>
// / <param name="img">Destination image.</param>
// / <param name="g">Game state.</param>
func drawSelect(img *ebiten.Image, g *Game) {
	text.Draw(img, "Selecciona las IA que jugaran", midFace, 40, 62, colLine)

	step := "Blancas"
	if g.selStep == 1 {
		step = "Negras"
	}
	text.Draw(img, "Haz clic en una red para asignarla a: "+step+".", smallFace, 40, 92, colDim)

	wColor := colDim
	if g.selStep == 0 {
		wColor = colLine
	}
	bColor := colDim
	if g.selStep == 1 {
		bColor = colLine
	}
	text.Draw(img, "Blancas: "+g.selName(g.selWhite), smallFace, 40, 122, wColor)
	text.Draw(img, "Negras:  "+g.selName(g.selBlack), smallFace, 40, 144, bColor)

	if len(g.files) == 0 {
		text.Draw(img, "No se encontraron archivos .bin de pesos junto al ejecutable.", smallFace, 40, selListY+20, colCheck)
		return
	}

	for i, p := range g.files {
		x, y, w, h := selRowRect(i)
		bg := selRowBg
		if i == g.selCursor {
			bg = selRowCursor
		}
		fillRect(img, x, y, w, h, bg)
		text.Draw(img, baseName(p), smallFace, x+16, y+h*2/3+2, colLine)
		if i == g.selWhite {
			fillRect(img, x+w-58, y+6, 26, h-12, selWhiteTag)
			text.Draw(img, "B", midFace, x+w-52, y+h*2/3+4, colBg)
		}
		if i == g.selBlack {
			fillRect(img, x+w-28, y+6, 26, h-12, selBlackTag)
			text.Draw(img, "N", midFace, x+w-23, y+h*2/3+4, colBg)
		}
	}

	x, y, w, h := g.startRect()
	bg := selStartOff
	if g.ready() {
		bg = selStartBg
	}
	fillRect(img, x, y, w, h, bg)
	text.Draw(img, "JUGAR", midFace, x+w/2-52, y+h/2+11, colLine)

	text.Draw(img, "Clic = elegir    R = aleatorio    Enter = jugar (si ambas elegidas)    Esc = salir",
		smallFace, 40, screenH-24, colDim)
}
