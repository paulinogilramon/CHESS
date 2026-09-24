///
/// Package main implements the Ebitengine-based graphical chess client.
/// It reuses the engine package for state and move generation and only
/// adds presentation and input handling.
package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chess/engine"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

///
/// <summary>
///   autoMode enables autonomous AI vs AI play; the recent weights play White
///   and the previous weights play Black so the matchup is visible.
/// </summary>
var (
	autoMode  bool
	whiteCfg  engine.NNConfig
	blackCfg  engine.NNConfig
	whiteName string
	blackName string
)

///
/// <summary>
///   init resolves the game mode. When the executable is named gui_ia_ia (or
///   CHESS_AUTOPLAY is set) both sides are engines: weights.bin (White) vs
///   weights_old.bin (Black). Otherwise the classic setup is preserved and
///   the loaded net is installed as the default engine evaluation.
/// </summary>
func init() {
	autoMode = isAutoExe()

	wPath := engineEnvOr("CHESS_WHITE_WEIGHTS", weightsPath())
	bPath := engineEnvOr("CHESS_BLACK_WEIGHTS", wPath)
	if autoMode {
		if p := assetOrCwd("weights_old.bin"); fileExists(p) {
			bPath = engineEnvOr("CHESS_BLACK_WEIGHTS", p)
		}
	}
	whiteName = baseName(wPath)
	blackName = baseName(bPath)

	loadCfg := func(path string) engine.NNConfig {
		if _, err := os.Stat(path); err != nil {
			log.Println("neural weights not found:", path)
			return engine.NNConfig{}
		}
		net, err := engine.LoadNN(path, 0.6, 600)
		if err != nil {
			log.Println("neural weights load failed:", path, err)
			return engine.NNConfig{}
		}
		return engine.NNConfig{Net: net, Blend: 0.6, Scale: 600}
	}
	whiteCfg = loadCfg(wPath)
	blackCfg = loadCfg(bPath)
	if autoMode {
		engine.SetExploration(0.5)
		log.Println("exploration on")
	}
	if !autoMode && blackCfg.Net != nil {
		engine.SetDefaultNN(blackCfg)
		log.Println("neural evaluation loaded")
	}
}

///
/// <summary>
///   isAutoExe detects whether this build should run the AI vs AI showcase.
/// </summary>
/// <returns>True when launched as gui_ia_ia or CHESS_AUTOPLAY is set.</returns>
func isAutoExe() bool {
	if os.Getenv("CHESS_AUTOPLAY") != "" {
		return true
	}
	if exe, err := os.Executable(); err == nil {
		if strings.Contains(strings.ToLower(filepath.Base(exe)), "ia_ia") {
			return true
		}
	}
	return false
}

/// <summary>
///   autoOpenMax is the number of random opening plies before the engines
///   start searching, so AI vs AI games differ every match.
/// </summary>
const autoOpenMax = 5

///
/// <summary>
///   weightsPath resolves weights.bin next to the executable, falling back to
///   the working directory so the game works regardless of where it is run.
/// </summary>
/// <returns>The resolved weights file path.</returns>
func weightsPath() string {
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), "weights.bin")
		if fileExists(p) {
			return p
		}
	}
	return "weights.bin"
}

///
/// <summary>
///   engineEnvOr returns the environment value when set, otherwise the default.
/// </summary>
/// <param name="key">Environment variable name.</param>
/// <param name="def">Fallback value.</param>
/// <returns>The environment value or the fallback.</returns>
func engineEnvOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

///
/// <summary>
///   assetOrCwd resolves a file next to the executable, falling back to the
///   working directory so builds run regardless of launch folder.
/// </summary>
/// <param name="name">File name to resolve.</param>
/// <returns>The resolved path.</returns>
func assetOrCwd(name string) string {
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), name)
		if fileExists(p) {
			return p
		}
	}
	return name
}

///
/// <summary>
///   fileExists reports whether a path exists.
/// </summary>
/// <param name="p">Path to check.</param>
/// <returns>True when the path exists.</returns>
func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

///
/// <summary>
///   baseName trims the file name for display in the UI.
/// </summary>
/// <param name="p">Full path.</param>
/// <returns>The base file name.</returns>
func baseName(p string) string {
	return filepath.Base(p)
}

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
	keys      []string
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
	auto      bool
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
		aiSide:   engine.Black,
		auto:     autoMode,
	}
	g.resetLogs()
	if g.auto {
		g.randomizeOpening()
	}
	return g
}

///
/// <summary>
///   randomizeOpening plays a short sequence of random legal moves from the
///   starting position so AI vs AI games do not repeat the same line.
/// </summary>
func (g *Game) randomizeOpening() {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	n := rng.Intn(autoOpenMax + 1)
	for i := 0; i < n; i++ {
		moves := engine.GenerateLegal(g.board)
		if len(moves) == 0 {
			break
		}
		engine.MakeMove(g.board, moves[rng.Intn(len(moves))])
	}
}

///
/// <summary>
///   resetLogs clears the transient status and log lines shown in the UI.
/// </summary>
func (g *Game) resetLogs() {
	if g.auto {
		g.status = fmt.Sprintf("AI vs AI: %s (White) vs %s (Black).", whiteName, blackName)
		return
	}
	g.logs = g.logs[:0]
	g.status = "New game. You are White; the engine plays Black."
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
	g.keys = g.keys[:0]
	g.keys = append(g.keys, engine.PositionKey(g.board))
	g.selected = -1
	g.dests = nil
	g.anim = nil
	g.lastFrom, g.lastTo = -1, -1
	g.resetLogs()
	if g.auto {
		g.randomizeOpening()
	}
}

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
	if len(g.keys) > 0 { // pop the key pushed for the undone ply
		g.keys = g.keys[:len(g.keys)-1]
	}
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
	g.keys = append(g.keys, engine.PositionKey(g.board))
	g.lastFrom, g.lastTo = m.From(), m.To()
	mover := engine.ColorOf(g.board.PieceAt(m.To()))
	g.anim = &animPiece{from: m.From(), to: m.To(), typ: engine.TypeOf(g.board.PieceAt(m.To())), col: mover}
	g.selected = -1
	g.dests = nil
	g.logs = append(g.logs, san)
	if len(g.logs) > 12 {
		g.logs = g.logs[1:]
	}
	g.status = g.statusText(g.board, san)
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
///   turn, and is a no-op otherwise. In auto mode both sides are engines,
///   each with its own weights.
/// </summary>
func (g *Game) engineMove() {
	if g.thinking {
		return
	}
	if g.anim != nil {
		return
	}
	if len(engine.GenerateLegal(g.board)) == 0 {
		return
	}
	if !g.auto {
		if g.aiSide == 0 || g.board.Stm != g.aiSide {
			return
		}
	}
	if g.auto && g.autoEnded() {
		g.status = "Match over by draw rule."
		return
	}
	g.thinking = true
	g.status = "Thinking..."
	g.aiCh = make(chan engine.Move, 1)
	go func() {
		clone := *g.board
		var cfg engine.NNConfig
		if g.auto {
			if clone.Stm == engine.White {
				cfg = whiteCfg
			} else {
				cfg = blackCfg
			}
		} else {
			cfg = engine.DefaultNNConfig()
		}
		g.aiCh <- engine.FindBestMoveWith(&clone, 4, 1200, cfg)
	}()
}

///
/// <summary>
///   autoEnded reports whether the automatic match reached a terminal state
///   (draw rules or checkmate) so the engines stop playing.
/// </summary>
/// <returns>True when the position is over.</returns>
func (g *Game) autoEnded() bool {
	if engine.FiftyMoveDraw(g.board) || engine.InsufficientMaterial(g.board) || g.repetitions() >= 3 {
		return true
	}
	return false
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
func (g *Game) statusText(s *engine.State, lastSan string) string {
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
	if engine.FiftyMoveDraw(s) {
		line = "Draw by the fifty-move rule."
	} else if engine.InsufficientMaterial(s) {
		line = "Draw by insufficient material."
	} else if g.repetitions() >= 3 {
		line = "Draw by threefold repetition."
	}
	if line == "" {
		line = "Move " + lastSan + ". " + colorWord(s.Stm) + " to move."
	}
	return line
}

///
/// <summary>
///   repetitions counts how many times the current position appears in the
///   game, which implements the threefold-repetition rule.
/// </summary>
/// <returns>The number of occurrences of the current position key.</returns>
func (g *Game) repetitions() int {
	n := 0
	cur := engine.PositionKey(g.board)
	for _, k := range g.keys {
		if k == cur {
			n++
		}
	}
	return n
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
	if g.auto {
		ebiten.SetWindowTitle("Chess - AI vs AI")
	} else {
		ebiten.SetWindowTitle("Chess")
	}
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
