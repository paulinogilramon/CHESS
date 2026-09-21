///
/// Package main implements the Ebitengine-based graphical chess client.
/// It reuses the engine package for state and move generation and only
/// adds presentation and input handling.
package main

import (
	"fmt"
	"log"

	"chess/engine"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

///
/// <summary>
///   Board geometry constants controlling the fixed logical screen size.
/// </summary>
const (
	screenW  = 640
	screenH  = 760
	cell     = 60
	cellF    = float64(cell)
	boardLen = cell * 8
	boardX   = (screenW - boardLen) / 2
	boardY   = 96
	statusY  = 40
	infoY    = screenH - 120
)

///
/// <summary>
///   histMove records a played move and the undo data needed to reverse it.
/// </summary>
type histMove struct {
	m   engine.Move
	u   engine.Undo
	san string
}

///
/// <summary>
///   animPiece describes a piece mid-slide between two squares.
/// </summary>
type animPiece struct {
	from, to int
	typ      int8
	col      engine.Color
	t        float64
}

///
/// <summary>
///   Game holds the current position, history, selection state and the
///   animation in-flight for the graphical client.
/// </summary>
type Game struct {
	board     *engine.State
	history   []histMove
	selected  int
	dests     []engine.Move
	movesHint bool
	anim      *animPiece
	lastFrom  int
	lastTo    int
	quit      bool
	fen       bool
	status    string
	logs      []string
	aiSide    engine.Color
	thinking  bool
	aiCh      chan engine.Move
}

///
/// <summary>
///   newGame returns a Game on the starting position.
/// </summary>
/// <returns>The initialized Game.</returns>
func newGame() *Game {
	g := &Game{
		board:    engine.NewStart(),
		selected: -1,
		lastFrom: -1,
		lastTo:   -1,
	}
	g.resetLogs()
	return g
}

///
/// <summary>
///   resetLogs clears the transient status and log lines shown in the UI.
/// </summary>
func (g *Game) resetLogs() {
	g.logs = g.logs[:0]
	g.status = "New game. Left-click a piece to see its moves."
}

///
/// <summary>
///   Reset restarts the game from the initial position.
/// </summary>
func (g *Game) Reset() {
	if g.thinking {
		g.status = "Wait for the engine."
		return
	}
	g.board = engine.NewStart()
	g.history = g.history[:0]
	g.selected = -1
	g.dests = nil
	g.anim = nil
	g.lastFrom, g.lastTo = -1, -1
	g.resetLogs()
}

///
/// <summary>
///   Undo takes back the most recent move, if any.
/// </summary>
func (g *Game) Undo() {
	if g.thinking {
		g.status = "Wait for the engine."
		return
	}
	if len(g.history) == 0 {
		g.status = "Nothing to undo."
		return
	}
	last := g.history[len(g.history)-1]
	engine.UndoMove(g.board, last.m, last.u)
	g.history = g.history[:len(g.history)-1]
	g.selected = -1
	g.dests = nil
	g.anim = nil
	g.lastFrom, g.lastTo = -1, -1
	if len(g.history) > 0 {
		g.lastFrom = g.history[len(g.history)-1].m.From()
		g.lastTo = g.history[len(g.history)-1].m.To()
	}
	g.status = fmt.Sprintf("Undid %s.", last.san)
}

///
/// <summary>
///   Update handles all input and animation stepping each frame.
/// </summary>
/// <returns>An error to terminate the application, or nil.</returns>
func (g *Game) Update() error {
	g.handleKeys()
	if g.quit {
		return ebiten.Termination
	}
	g.handleMouse()

	if g.anim != nil {
		g.anim.t += 1.0 / 60.0
		if g.anim.t >= 1 {
			g.anim = nil
		}
	}
	g.engineMove()
	if g.thinking {
		select {
		case mv := <-g.aiCh:
			g.thinking = false
			if mv != 0 && engine.CanMove(g.board, mv) {
				g.play(mv)
			} else {
				g.status = "Engine produced no move; play on."
			}
		default:
		}
	}
	return nil
}

///
/// <summary>
///   handleKeys processes keyboard shortcuts.
/// </summary>
func (g *Game) handleKeys() {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.selected = -1
		g.dests = nil
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyN) {
		g.Reset()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyU) {
		g.Undo()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyM) {
		g.movesHint = !g.movesHint
		g.dests = nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF) {
		g.fen = !g.fen
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyA) {
		if g.thinking {
			g.status = "Engine is still thinking."
			return
		}
		if g.aiSide == 0 {
			g.aiSide = g.board.Stm
			g.status = "Engine takes " + colorWord(g.aiSide) + "."
		} else {
			g.aiSide = 0
			g.status = "Engine off; both sides are human."
		}
	}
	if ebiten.IsKeyPressed(ebiten.KeyControl) && inpututil.IsKeyJustPressed(ebiten.KeyQ) {
		g.quit = true
	}
}

///
/// <summary>
///   handleMouse resolves clicks into square selection, move application,
///   and promotion choices.
/// </summary>
func (g *Game) handleMouse() {
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return
	}
	if g.thinking {
		return
	}
	mx, my := ebiten.CursorPosition()
	if g.anim != nil {
		return
	}
	if g.handlePromoClick(mx, my) {
		return
	}
	sq, ok := pixelToSquare(mx, my)
	if !ok {
		g.selected = -1
		g.dests = nil
		return
	}
	if g.selected >= 0 {
		for _, m := range g.dests {
			if m.To() == sq {
				g.applyMove(m.From(), sq)
				return
			}
		}
		g.selected = -1
		g.dests = nil
		return
	}
	if p := g.board.PieceAt(sq); p != 0 && engine.ColorOf(p) == g.board.Stm {
		g.selected = sq
		g.dests = engine.LegalDestinations(g.board, sq)
	}
}

///
/// <summary>
///   applyMove resolves the concrete move from a source to destination,
///   opening a promotion dialog when needed, and otherwise plays it.
/// </summary>
/// <param name="from">Source square.</param>
/// <param name="to">Destination square.</param>
func (g *Game) applyMove(from, to int) {
	g.selected = -1
	g.dests = nil
	found := engine.LegalDestinations(g.board, from)
	var chosen *engine.Move
	for _, m := range found {
		if m.To() == to {
			mm := m
			chosen = &mm
			if m.Promo() != 0 {
				g.beginPromo(from, to)
				return
			}
			break
		}
	}
	if chosen != nil {
		g.play(*chosen)
	}
}

///
/// <summary>
///   play performs a validated move on the board and records history.
/// </summary>
/// <param name="m">Legal move to play.</param>
func (g *Game) play(m engine.Move) {
	san := engine.San(g.board, m)
	u := engine.MakeMove(g.board, m)
	g.history = append(g.history, histMove{m, u, san})
	g.lastFrom, g.lastTo = m.From(), m.To()
	mover := engine.ColorOf(g.board.PieceAt(m.To()))
	g.anim = &animPiece{from: m.From(), to: m.To(), typ: engine.TypeOf(g.board.PieceAt(m.To())), col: mover}
	g.selected = -1
	g.dests = nil
	g.logs = append(g.logs, san)
	if len(g.logs) > 12 {
		g.logs = g.logs[1:]
	}
	g.status = statusText(g.board, san)
}

///
/// <summary>
///   beginPromo stores the pending promotion origin so a dialog can choose
///   the piece.
/// </summary>
/// <param name="from">Source square.</param>
/// <param name="to">Promotion destination.</param>
func (g *Game) beginPromo(from, to int) {
	g.selected = from
	g.dests = append(g.dests[:0], engine.LegalDestinations(g.board, from)...)
	g.status = "Choose a promotion piece."
}

///
/// <summary>
///   handlePromoClick resolves promotion choices; returns true when the click
///   was consumed by the dialog.
/// </summary>
/// <param name="mx">Logical x cursor.</param>
/// <param name="my">Logical y cursor.</param>
/// <returns>True when the promo dialog handled the click.</returns>
func (g *Game) handlePromoClick(mx, my int) bool {
	if g.selected < 0 || len(g.dests) == 0 {
		return false
	}
	hasPromo := false
	for _, d := range g.dests {
		if d.To() == g.lastPromoTarget() && d.Promo() != 0 {
			hasPromo = true
		}
	}
	if !hasPromo {
		return false
	}
	idx := promoSlot(mx, my)
	if idx < 0 {
		return false
	}
	order := []int8{engine.Queen, engine.Rook, engine.Bishop, engine.Knight}
	for _, m := range g.dests {
		if m.To() == g.lastPromoTarget() && m.Promo() == order[idx] {
			g.play(m)
		}
	}
	return true
}

///
/// <summary>
///   lastPromoTarget finds the destination of a pending promotion move.
/// </summary>
/// <returns>Destination square, or -1.</returns>
func (g *Game) lastPromoTarget() int {
	if g.selected < 0 {
		return -1
	}
	for _, d := range g.dests {
		if d.From() == g.selected && d.Promo() != 0 {
			return d.To()
		}
	}
	return -1
}

///
/// <summary>
///   engineMove fires an asynchronous search when it is the engine side's
///   turn, and is a no-op otherwise.
/// </summary>
func (g *Game) engineMove() {
	if g.aiSide == 0 || g.thinking {
		return
	}
	if g.anim != nil || g.board.Stm != g.aiSide {
		return
	}
	if len(engine.GenerateLegal(g.board)) == 0 {
		return
	}
	g.thinking = true
	g.status = "Engine thinking..."
	g.aiCh = make(chan engine.Move, 1)
	go func() {
		g.aiCh <- engine.FindBestMove(g.board, 4, 1200)
	}()
}

///
/// <summary>
///   Layout fixes the logical screen resolution; the window may scale.
/// </summary>
/// <param name="outsideWidth">Window width in physical pixels.</param>
/// <param name="outsideHeight">Window height in physical pixels.</param>
/// <returns>The fixed logical size.</returns>
func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenW, screenH
}

///
/// <summary>
///   statusText builds the status bar line after a move.
/// </summary>
/// <param name="s">Position to describe.</param>
/// <param name="lastSan">SAN of the move just played.</param>
/// <returns>The status line.</returns>
func statusText(s *engine.State, lastSan string) string {
	line := "Move " + lastSan + ". " + colorWord(s.Stm) + " to move."
	if len(engine.GenerateLegal(s)) == 0 {
		if engine.IsInCheck(s, s.Stm) {
			line = "Checkmate - " + winnerWord(s) + " wins!"
		} else {
			line = "Stalemate - draw."
		}
	} else if engine.IsInCheck(s, s.Stm) {
		line += " Check!"
	}
	return line
}

///
/// <summary>
///   colorWord renders a side's name.
/// </summary>
/// <param name="c">Color.</param>
/// <returns>"White" or "Black".</returns>
func colorWord(c engine.Color) string {
	if c == engine.White {
		return "White"
	}
	return "Black"
}

///
/// <summary>
///   winnerWord names the side that delivered checkmate.
/// </summary>
/// <param name="s">Position.</param>
/// <returns>The winning side.</returns>
func winnerWord(s *engine.State) string {
	if s.Stm == engine.White {
		return "Black"
	}
	return "White"
}

///
/// <summary>
///   sortedLogs renders recent move logs with move numbers for the footer.
/// </summary>
/// <returns>A single line of move SAN text.</returns>
func (g *Game) sortedLogs() string {
	line := ""
	for i, san := range g.logs {
		if i > 0 {
			line += " "
		}
		line += san
	}
	return line
}


func main() {
	g := newGame()
	ebiten.SetWindowSize(screenW, screenH)
	ebiten.SetWindowTitle("Chess")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}