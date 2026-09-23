///
/// Command chess is a console chess client built on the engine package.
/// It provides a REPL for playing, inspecting, and setting positions.
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"chess/engine"
)

///
/// <summary>
///   init loads the trained neural evaluator from weights.bin when present and
///   installs it as the default engine evaluation, blending 60% neural with
///   the classical score at a 600-cp scale, matching the GUI client.
/// </summary>
func init() {
	if _, err := os.Stat(weightsPath()); err != nil {
		fmt.Println("neural weights not found; using classical evaluation")
		return
	}
	net, err := engine.LoadNN(weightsPath(), 0.6, 600)
	if err != nil {
		fmt.Println("neural weights load failed:", err)
		return
	}
	engine.SetDefaultNN(engine.NNConfig{Net: net, Blend: 0.6, Scale: 600})
	fmt.Println("neural evaluation loaded")
}

///
/// <summary>
///   weightsPath resolves weights.bin next to the executable, falling back to
///   the working directory so the game works regardless of where it is run.
/// </summary>
/// <returns>The resolved weights file path.</returns>
func weightsPath() string {
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), "weights.bin")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "weights.bin"
}

///
/// <summary>
///   movesPlayed records a move together with the undo info needed to
///   reverse it, forming the position history stack.
/// </summary>
type movesPlayed struct {
	m engine.Move
	u engine.Undo
	///
	/// <summary>
	///   san is the move's SAN text for display in the history.
	/// </summary>
	san string
}

func main() {
	s := engine.NewStart()
	history := make([]movesPlayed, 0, 128)
	sc := bufio.NewScanner(os.Stdin)

	welcome()
	for {
		maybeEnd(s)
		fmt.Printf("\n[%d] %s> ", s.Fullmove, colorWord(s.Stm))
		if !sc.Scan() {
			break
		}
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		args := strings.Fields(line)
		cmd := strings.ToLower(args[0])
		rest := strings.Join(args[1:], " ")

		switch cmd {
		case "quit", "exit", "q":
			fmt.Println("bye")
			return
		case "help", "?", "h":
			help()
		case "new":
			s = engine.NewStart()
			history = history[:0]
			fmt.Println("New game.")
			show(s)
		case "show", "board", "b":
			show(s)
		case "fen":
			fmt.Println(engine.ToFen(s))
		case "setfen", "fen-set":
			parsed, err := engine.ParseFen(rest)
			if err != nil {
				fmt.Println("err:", err)
				break
			}
			s = parsed
			history = history[:0]
			fmt.Println("Position set.")
			show(s)
		case "moves", "m":
			printMoves(s)
		case "undo", "u":
			if len(history) == 0 {
				fmt.Println("Nothing to undo.")
				break
			}
			last := history[len(history)-1]
			engine.UndoMove(s, last.m, last.u)
			history = history[:len(history)-1]
			fmt.Println("Undid one move.")
			show(s)
		case "hist", "history":
			printHistory(history)
		case "go":
			m := engine.FindBestMove(s, 4, 1500)
			san := engine.San(s, m)
			u := engine.MakeMove(s, m)
			history = append(history, movesPlayed{m, u, san})
			fmt.Println("Engine plays:", san)
			show(s)
		default:
			if m, ok := engine.ParseSan(s, line); ok {
				san := engine.San(s, m)
				u := engine.MakeMove(s, m)
				history = append(history, movesPlayed{m, u, san})
				show(s)
			} else {
				fmt.Printf("Unknown command or illegal move: %q\n", line)
			}
		}
	}
}

///
/// <summary>
///   welcome prints the startup banner and command hint.
/// </summary>
func welcome() {
	fmt.Println("Console Chess  -  type `help` for commands, `quit` to exit.")
}

///
/// <summary>
///   help prints the available commands.
/// </summary>
func help() {
	fmt.Println(`
Commands:
  new                 start a new game
  show                display the board
  moves               list all legal moves (SAN)
  hist                list moves already played
  undo                take back the last move
  go                  engine plays its best move
  fen [fenstring]     print FEN, or with an argument set the position
  e4 / Nf3 / O-O / e2e4q
                      play a move in SAN or coordinate notation
  quit                exit`)
}

///
/// <summary>
///   show prints the board and the position status.
/// </summary>
/// <param name="s">Position to display.</param>
func show(s *engine.State) {
	printBoard(s)
	fmt.Printf("Turn: %s  Castling: %s  En passant: %s  Halfmove: %d  Fullmove: %d\n",
		colorWord(s.Stm), castlingWord(s), epWord(s), s.Halfmove, s.Fullmove)
	moves := engine.GenerateLegal(s)
	switch {
	case len(moves) == 0 && engine.IsInCheck(s, s.Stm):
		fmt.Println("Checkmate. Winner:", colorWord(-s.Stm))
	case len(moves) == 0:
		fmt.Println("Stalemate.")
	case engine.IsInCheck(s, s.Stm):
		fmt.Printf("Check! %d legal moves available.\n", len(moves))
	default:
		fmt.Printf("%d legal moves.\n", len(moves))
	}
}

///
/// <summary>
///   printBoard renders the board with files and ranks, White at the
///   bottom like an over-the-board view.
/// </summary>
/// <param name="s">Position to display.</param>
func printBoard(s *engine.State) {
	fmt.Println("   a b c d e f g h")
	for r := 7; r >= 0; r-- {
		fmt.Printf("%d ", r+1)
		for f := 0; f < 8; f++ {
			fmt.Printf(" %c", drawPiece(s.Board[engine.Sq(f, r)]))
		}
		fmt.Printf(" %d\n", r+1)
	}
	fmt.Println("   a b c d e f g h")
}

///
/// <summary>
///   drawPiece maps a stored piece value to its display character.
/// </summary>
/// <param name="p">Stored piece value.</param>
/// <returns>A single piece letter, or '.' for an empty square.</returns>
func drawPiece(p int8) rune {
	switch engine.TypeOf(p) {
	case engine.Pawn:
		if engine.ColorOf(p) == engine.White {
			return 'P'
		}
		return 'p'
	case engine.Knight:
		if engine.ColorOf(p) == engine.White {
			return 'N'
		}
		return 'n'
	case engine.Bishop:
		if engine.ColorOf(p) == engine.White {
			return 'B'
		}
		return 'b'
	case engine.Rook:
		if engine.ColorOf(p) == engine.White {
			return 'R'
		}
		return 'r'
	case engine.Queen:
		if engine.ColorOf(p) == engine.White {
			return 'Q'
		}
		return 'q'
	case engine.King:
		if engine.ColorOf(p) == engine.White {
			return 'K'
		}
		return 'k'
	}
	return '.'
}

///
/// <summary>
///   printMoves lists every legal move in SAN, grouped by row.
/// </summary>
/// <param name="s">Position whose legal moves are listed.</param>
func printMoves(s *engine.State) {
	legal := engine.GenerateLegal(s)
	for i, m := range legal {
		if i > 0 {
			fmt.Print(" ")
		}
		fmt.Print(engine.San(s, m))
	}
	fmt.Println()
}

///
/// <summary>
///   printHistory lists the moves already played in SAN pairs.
/// </summary>
/// <param name="h">History stack.</param>
func printHistory(h []movesPlayed) {
	for i, mp := range h {
		if i%2 == 0 {
			fmt.Printf("%3d. %-7s", i/2+1, mp.san)
		} else {
			fmt.Printf("%-7s\n", mp.san)
		}
	}
	if len(h)%2 == 1 {
		fmt.Println()
	}
}

///
/// <summary>
///   colorWord renders a color name.
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
///   castlingWord renders the castling rights field.
/// </summary>
/// <param name="s">Position.</param>
/// <returns>"KQkq", a subset, or "-".</returns>
func castlingWord(s *engine.State) string {
	if s.Castling == 0 {
		return "-"
	}
	var b strings.Builder
	if s.Castling&engine.WKS != 0 {
		b.WriteByte('K')
	}
	if s.Castling&engine.WQS != 0 {
		b.WriteByte('Q')
	}
	if s.Castling&engine.BKS != 0 {
		b.WriteByte('k')
	}
	if s.Castling&engine.BQS != 0 {
		b.WriteByte('q')
	}
	return b.String()
}

///
/// <summary>
///   epWord renders the en passant target square.
/// </summary>
/// <param name="s">Position.</param>
/// <returns>A square name, or "-".</returns>
func epWord(s *engine.State) string {
	if s.Ep < 0 {
		return "-"
	}
	f := byte('a' + engine.SqFile(int(s.Ep)))
	r := byte('1' + engine.SqRank(int(s.Ep)))
	return string([]byte{f, r})
}

///
/// <summary>
///   maybeEnd reports whether the game has concluded (mate or stalemate).
/// </summary>
/// <param name="s">Position to inspect.</param>
/// <returns>True when there are no legal moves.</returns>
func maybeEnd(s *engine.State) bool {
	if len(engine.GenerateLegal(s)) == 0 {
		if engine.IsInCheck(s, s.Stm) {
			fmt.Printf("== Checkmate: %s wins ==\n", colorWord(-s.Stm))
		} else {
			fmt.Println("== Stalemate ==")
		}
		return true
	}
	return false
}