package main

import (
	"image/color"
	"os"
	"path/filepath"

	"chess/engine"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/opentype"
)

///
/// <summary>
///   Palette colors used across the board, highlights and text.
/// </summary>
var (
	colLight   = color.RGBA{0xF0, 0xD9, 0xB5, 0xFF}
	colDark    = color.RGBA{0xB5, 0x88, 0x63, 0xFF}
	colBg      = color.RGBA{0x30, 0x2E, 0x2B, 0xFF}
	colPanel   = color.RGBA{0x1E, 0x1E, 0x1E, 0xD8}
	colSel     = color.RGBA{0xFF, 0xC8, 0x32, 0x80}
	colLast    = color.RGBA{0xFF, 0xE0, 0x7D, 0x99}
	colHint    = color.RGBA{0x2A, 0xC6, 0x5C, 0x4C}
	colDot     = color.RGBA{0x8F, 0xF0, 0xA7, 0xE0}
	colCheck   = color.RGBA{0xE8, 0x4B, 0x3A, 0xB0}
	colWhite   = color.RGBA{0xFF, 0xFF, 0xFF, 0xFF}
	colBlack   = color.RGBA{0x1F, 0x1F, 0x25, 0xFF}
	colOutline = color.RGBA{0x00, 0x00, 0x00, 0xB8}
	colLine    = color.RGBA{0xEE, 0xE8, 0xE0, 0xFF}
	colDim     = color.RGBA{0xA8, 0xA2, 0x9A, 0xFF}
)

///
/// <summary>
///   pieceFace and smallFace are the loaded font faces for piece glyphs and
///   UI text respectively.
/// </summary>
var (
	pieceFace font.Face
	smallFace font.Face
	midFace   font.Face
)

///
/// <summary>
///   loadFaces initializes the font stack, preferring system fonts that
///   contain the chess piece glyphs and falling back to a bitmap font.
/// </summary>
func loadFaces() {
	if pieceFace != nil {
		return
	}
	windir := os.Getenv("WINDIR")
	if windir == "" {
		windir = `C:\Windows`
	}
	candidates := []string{
		filepath.Join(windir, `Fonts\seguisym.ttf`),
		filepath.Join(windir, `Fonts\dejavusans.ttf`),
		filepath.Join(windir, `Fonts\segoeui.ttf`),
		filepath.Join(windir, `Fonts\arial.ttf`),
	}
	pieceFace = basicfont.Face7x13
	smallFace = basicfont.Face7x13
	midFace = basicfont.Face7x13
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		tt, err := opentype.Parse(data)
		if err != nil {
			continue
		}
		pieceFace, _ = opentype.NewFace(tt, &opentype.FaceOptions{Size: 44, DPI: 96, Hinting: font.HintingFull})
		pf, _ := opentype.NewFace(tt, &opentype.FaceOptions{Size: 34, DPI: 96, Hinting: font.HintingFull})
		midFace = pf
		sf, err := opentype.NewFace(tt, &opentype.FaceOptions{Size: 18, DPI: 96, Hinting: font.HintingFull})
		if err == nil {
			smallFace = sf
		}
		break
	}
}

///
/// <summary>
///   Draw renders every layer of the chess interface.
/// </summary>
/// <param name="screen">Destination image.</param>
func (g *Game) Draw(screen *ebiten.Image) {
	loadFaces()
	screen.Fill(colBg)
	drawSquares(screen, g)
	drawPieces(screen, g)
	drawAnim(screen, g)
	drawPromo(screen, g)
	drawStatus(screen, g)
	drawInfo(screen, g)
}

///
/// <summary>
///   drawSquares paints the board background, highlights and target hints.
/// </summary>
/// <param name="img">Destination image.</param>
/// <param name="g">Game state.</param>
func drawSquares(img *ebiten.Image, g *Game) {
	for row := 0; row < 8; row++ {
		for col := 0; col < 8; col++ {
			rank := 7 - row
			colr := colLight
			if (col+rank)%2 == 0 {
				colr = colDark
			}
			fillRect(img, boardX+col*cell, boardY+row*cell, cell, cell, colr)
		}
	}

	overlay := func(sq int, c color.RGBA) {
		fx, fy := squarePixel(sq)
		fillRect(img, fx, fy, cell, cell, c)
	}
	if g.lastFrom >= 0 {
		overlay(g.lastFrom, colLast)
	}
	if g.lastTo >= 0 {
		overlay(g.lastTo, colLast)
	}
	if g.selected >= 0 {
		overlay(g.selected, colSel)
	}
	for _, m := range g.dests {
		overlay(m.To(), colHint)
	}
	if engine.IsInCheck(g.board, g.board.Stm) {
		if k := g.board.KingSquare(g.board.Stm); k >= 0 {
			overlay(k, colCheck)
		}
	}
	if g.selected < 0 && g.movesHint {
		for _, m := range engine.GenerateLegal(g.board) {
			dotAt(img, m.To())
		}
	}
	for _, m := range g.dests {
		dotAt(img, m.To())
	}
}

///
/// <summary>
///   pieceSprite is a pre-rendered glyph image tagged with the offset from
///   its top-left corner to the center of its visible ink, so it can be
///   placed exactly in the middle of a square.
/// </summary>
type pieceSprite struct {
	img *ebiten.Image
	cx  float64
	cy  float64
}

///
/// <summary>
///   spriteKey identifies a piece sprite by type and color.
/// </summary>
type spriteKey struct {
	typ int8
	col engine.Color
}

///
/// <summary>
///   spriteCache memoizes built piece sprites across frames.
/// </summary>
var spriteCache = map[spriteKey]*pieceSprite{}

///
/// <summary>
///   getPieceSprite returns (building if needed) the centered glyph sprite
///   for a piece type and color.
/// </summary>
/// <param name="typ">Piece type.</param>
/// <param name="col">Piece color.</param>
/// <returns>The cached sprite.</returns>
func getPieceSprite(typ int8, col engine.Color) *pieceSprite {
	key := spriteKey{typ, col}
	if sp, ok := spriteCache[key]; ok {
		return sp
	}
	sp := buildPieceSprite(typ, col)
	spriteCache[key] = sp
	return sp
}

///
/// <summary>
///   buildPieceSprite renders a glyph with an outline and locates the visible
///   ink bounding box so the sprite centers its artwork, not its font metrics.
/// </summary>
/// <param name="typ">Piece type.</param>
/// <param name="col">Piece color.</param>
/// <returns>The built sprite.</returns>
func buildPieceSprite(typ int8, col engine.Color) *pieceSprite {
	str := string(glyphRune(typ, col))
	b, _ := font.BoundString(pieceFace, str)
	pad := 3
	w := (b.Max.X - b.Min.X).Ceil() + 2*pad
	h := (b.Max.Y - b.Min.Y).Ceil() + 2*pad
	img := ebiten.NewImage(w, h)
	img.Clear()
	baseY := pad - b.Min.Y.Floor()
	clr := colWhite
	if col != engine.White {
		clr = colBlack
	}
	for _, off := range [][2]int{{2, 0}, {-2, 0}, {0, 2}, {0, -2}} {
		text.Draw(img, str, pieceFace, pad+off[0], baseY+off[1], colOutline)
	}
	text.Draw(img, str, pieceFace, pad, baseY, clr)

	loX, loY := w, h
	hiX, hiY := 0, 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			if a == 0 {
				continue
			}
			if x < loX {
				loX = x
			}
			if x > hiX {
				hiX = x
			}
			if y < loY {
				loY = y
			}
			if y > hiY {
				hiY = y
			}
		}
	}
	return &pieceSprite{img: img, cx: float64(loX+hiX) / 2, cy: float64(loY+hiY) / 2}
}

///
/// <summary>
///   drawPieceAt draws a piece sprite centered at a given point.
/// </summary>
/// <param name="img">Destination image.</param>
/// <param name="sp">Sprite to draw.</param>
/// <param name="cx">Center x.</param>
/// <param name="cy">Center y.</param>
func drawPieceAt(img *ebiten.Image, sp *pieceSprite, cx, cy float64) {
	opts := &ebiten.DrawImageOptions{}
	opts.GeoM.Translate(cx-sp.cx, cy-sp.cy)
	img.DrawImage(sp.img, opts)
}

///
/// <summary>
///   drawPieces paints the static board contents as centered glyph sprites.
/// </summary>
/// <param name="img">Destination image.</param>
/// <param name="g">Game state.</param>
func drawPieces(img *ebiten.Image, g *Game) {
	for sq, p := range g.board.Board {
		if p == 0 {
			continue
		}
		if g.anim != nil && sq == g.anim.to {
			continue
		}
		fx, fy := squarePixel(sq)
		drawPieceAt(img, getPieceSprite(engine.TypeOf(p), engine.ColorOf(p)), float64(fx)+cellF/2, float64(fy)+cellF/2)
	}
}

///
/// <summary>
///   drawAnim paints the sliding piece during a move animation.
/// </summary>
/// <param name="img">Destination image.</param>
/// <param name="g">Game state.</param>
func drawAnim(img *ebiten.Image, g *Game) {
	a := g.anim
	if a == nil {
		return
	}
	if a.t < 0 {
		a.t = 0
	}
	if a.t > 1 {
		a.t = 1
	}
	x1, y1 := squarePixel(a.from)
	x2, y2 := squarePixel(a.to)
	cx := float64(x2-x1)*a.t + float64(x1) + cellF/2
	cy := float64(y2-y1)*a.t + float64(y1) + cellF/2
	drawPieceAt(img, getPieceSprite(a.typ, a.col), cx, cy)
}

///
/// <summary>
///   drawPromo paints the promotion chooser overlay when a pawn move needs a
///   piece selection.
/// </summary>
/// <param name="img">Destination image.</param>
/// <param name="g">Game state.</param>
func drawPromo(img *ebiten.Image, g *Game) {
	target := g.lastPromoTarget()
	if g.selected < 0 || target < 0 {
		return
	}
	mover := g.board.Stm
	order := []int8{engine.Queen, engine.Rook, engine.Bishop, engine.Knight}

	gap := 10
	total := 4*cell + 3*gap
	x0 := (screenW - total) / 2
	y0 := boardY + (boardLen-cell)/2 - cell/2
	fillRect(img, x0-20, y0-52, total+40, cell+68, colPanel)
	text.Draw(img, "Promote to", midFace, x0-8, y0-24, colLine)
	for i, typ := range order {
		sx := x0 + i*(cell+gap)
		fillRect(img, sx, y0, cell, cell, colLight)
		if i%2 == 0 {
			fillRect(img, sx, y0, cell, cell, colDark)
		}
		drawPieceAt(img, getPieceSprite(typ, mover), float64(sx)+cellF/2, float64(y0)+cellF/2)
	}
}

///
/// <summary>
///   promoLayout computes the placement of the promotion dialog.
/// </summary>
/// <param name="x">Output left coordinate.</param>
/// <param name="y">Output top coordinate.</param>
/// <returns>True (layout always provided).</returns>
func promoLayout(x, y *int) bool {
	gap := 10
	total := 4*cell + 3*gap
	*x = (screenW - total) / 2
	*y = boardY + (boardLen-cell)/2 - cell/2
	return true
}

///
/// <summary>
///   drawStatus renders the top status line describing the position.
/// </summary>
/// <param name="img">Destination image.</param>
/// <param name="g">Game state.</param>
func drawStatus(img *ebiten.Image, g *Game) {
	header := colorWord(g.board.Stm)
	if engine.IsInCheck(g.board, g.board.Stm) {
		header += " in check"
	}
	text.Draw(img, header, midFace, 24, statusY+18, colLine)
	text.Draw(img, g.status, smallFace, 24, statusY+44, colDim)
}

///
/// <summary>
///   drawInfo renders the footer with move history and keyboard help.
/// </summary>
/// <param name="img">Destination image.</param>
/// <param name="g">Game state.</param>
func drawInfo(img *ebiten.Image, g *Game) {
	text.Draw(img, g.sortedLogs(), smallFace, 24, infoY, colLine)
	help := "n=new  u=undo  m=hints  f=fen  Esc=clear  Ctrl+Q=quit"
	text.Draw(img, help, smallFace, 24, infoY+28, colDim)
	if g.fen {
		text.Draw(img, engine.ToFen(g.board), smallFace, 24, infoY+54, colLine)
	}
}

///
/// <summary>
///   fillRect paints a solid rectangle on the image.
/// </summary>
/// <param name="img">Destination image.</param>
/// <param name="x">Left coordinate.</param>
/// <param name="y">Top coordinate.</param>
/// <param name="w">Width.</param>
/// <param name="h">Height.</param>
/// <param name="c">Fill color.</param>
func fillRect(img *ebiten.Image, x, y, w, h int, c color.RGBA) {
	r := ebiten.NewImage(w, h)
	r.Fill(c)
	opts := &ebiten.DrawImageOptions{}
	opts.GeoM.Translate(float64(x), float64(y))
	img.DrawImage(r, opts)
}

///
/// <summary>
///   squarePixel maps a square index to its pixel origin.
/// </summary>
/// <param name="sq">Square index.</param>
/// <returns>The pixel x/y top-left corner.</returns>
func squarePixel(sq int) (int, int) {
	file := engine.SqFile(sq)
	rank := engine.SqRank(sq)
	row := 7 - rank
	return boardX + file*cell, boardY + row*cell
}

///
/// <summary>
///   pixelToSquare maps a cursor position to a board square.
/// </summary>
/// <param name="mx">Logical x.</param>
/// <param name="my">Logical y.</param>
/// <returns>The square index and whether the click landed on the board.</returns>
func pixelToSquare(mx, my int) (int, bool) {
	col := (mx - boardX) / cell
	row := (my - boardY) / cell
	if col < 0 || col > 7 || row < 0 || row > 7 {
		return 0, false
	}
	return engine.Sq(col, 7-row), true
}

///
/// <summary>
///   glyphRune maps a piece type and color to its Unicode symbol.
/// </summary>
/// <param name="typ">Piece type.</param>
/// <param name="c">Piece color.</param>
/// <returns>The piece glyph rune.</returns>
func glyphRune(typ int8, c engine.Color) rune {
	var base rune
	if c == engine.White {
		base = 0x2654
	} else {
		base = 0x265A
	}
	switch typ {
	case engine.King:
		return base
	case engine.Queen:
		return base + 1
	case engine.Rook:
		return base + 2
	case engine.Bishop:
		return base + 3
	case engine.Knight:
		return base + 4
	case engine.Pawn:
		return base + 5
	}
	return rune('?')
}

///
/// <summary>
///   dotAt paints a small target marker at the center of a square.
/// </summary>
/// <param name="img">Destination image.</param>
/// <param name="sq">Square to mark.</param>
func dotAt(img *ebiten.Image, sq int) {
	fx, fy := squarePixel(sq)
	d := cell / 4
	fillRect(img, fx+cell/2-d/2, fy+cell/2-d/2, d, d, colDot)
}

///
/// <summary>
///   promoSlot resolves a click into the promotion piece slot index.
/// </summary>
/// <param name="mx">Logical x.</param>
/// <param name="my">Logical y.</param>
/// <returns>0..3 for Q/R/B/N, or -1 when outside the dialog.</returns>
func promoSlot(mx, my int) int {
	var x0, y0 int
	promoLayout(&x0, &y0)
	gap := 10
	for i := 0; i < 4; i++ {
		sx := x0 + i*(cell+gap)
		if mx >= sx && mx < sx+cell && my >= y0 && my < y0+cell {
			return i
		}
	}
	return -1
}