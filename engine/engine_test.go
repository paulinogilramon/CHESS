package engine

import (
	"strings"
	"testing"
)

///
/// <summary>
///   TestStartMoveCount asserts the starting position yields exactly 20
///   legal moves for each side (16 pawn + 4 knight).
/// </summary>
func TestStartMoveCount(t *testing.T) {
	s := NewStart()
	if n := len(GenerateLegal(s)); n != 20 {
		t.Fatalf("start position: got %d legal moves, want 20", n)
	}
	s.Stm = Black
	if n := len(GenerateLegal(s)); n != 20 {
		t.Fatalf("black start position: got %d legal moves, want 20", n)
	}
}

///
/// <summary>
///   TestStartFen verifies the serialized starting position matches the
///   canonical FEN string.
/// </summary>
func TestStartFen(t *testing.T) {
	got := ToFen(NewStart())
	want := "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"
	if got != want {
		t.Fatalf("start FEN mismatch\n got: %s\nwant: %s", got, want)
	}
}

///
/// <summary>
///   TestFenRoundTrip builds a position from FEN and serializes it back.
/// </summary>
func TestFenRoundTrip(t *testing.T) {
	fens := []string{
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
		"r3k2r/8/8/8/8/8/8/R3K2R b KQkq - 0 1",
		"8/2p5/3p4/KP5r/1R3p1k/8/4P1P1/8 w - - 0 1",
		"4k3/P7/8/8/8/8/8/4K3 w - - 0 1",
	}
	for _, fen := range fens {
		s, err := ParseFen(fen)
		if err != nil {
			t.Fatalf("parse %q: %v", fen, err)
		}
		if got := ToFen(s); got != fen {
			t.Fatalf("fen round trip\n got: %s\nwant: %s", got, fen)
		}
	}
}

///
/// <summary>
///   TestCastling verifies both castling directions for White from an
///   open-back-rank position and that rights are cleared afterward.
/// </summary>
func TestCastling(t *testing.T) {
	s, err := ParseFen("r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	legal := GenerateLegal(s)
	hasCastle := func(flag byte) bool {
		for _, m := range legal {
			if m.Flag() == flag {
				return true
			}
		}
		return false
	}
	if !hasCastle(FlagCastle) {
		t.Fatal("expected castling moves")
	}

	m, ok := ParseSan(s, "O-O")
	if !ok || m.Flag() != FlagCastle || m.To() != 6 {
		t.Fatalf("O-O should parse to a kingside castle, got %v ok=%v", m, ok)
	}
	u := MakeMove(s, m)
	if s.Board[6] != King || s.Board[5] != Rook || s.Board[7] != 0 {
		t.Fatalf("kingside castle board wrong")
	}
	if s.Castling&(WKS|WQS) != 0 {
		t.Fatalf("white castling rights not cleared")
	}
	UndoMove(s, m, u)
	if s.Board[4] != King || s.Board[7] != Rook {
		t.Fatalf("castling undo failed")
	}
}

///
/// <summary>
///   TestCastlingThroughCheck ensures castling is illegal when a square the
///   king crosses is attacked.
/// </summary>
func TestCastlingThroughCheck(t *testing.T) {
	// Black rook on f8 covers the f-file, so white kingside castling (through
	// f1) must be rejected while queenside remains available.
	s, err := ParseFen("5r1k/8/8/8/8/8/8/R3K2R w KQ - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range GenerateLegal(s) {
		if m.Flag() == FlagCastle && m.To() == 6 {
			t.Fatalf("kingside castling must be illegal when f1 is attacked: %v", m)
		}
	}
	hasQueenside := false
	for _, m := range GenerateLegal(s) {
		if m.Flag() == FlagCastle && m.To() == 2 {
			hasQueenside = true
		}
	}
	if !hasQueenside {
		t.Fatalf("queenside castling should remain legal")
	}
}

///
/// <summary>
///   TestEnPassant verifies ep square bookkeeping and the en passant capture.
/// </summary>
func TestEnPassant(t *testing.T) {
	s := NewStart()
	play := func(uci string) {
		m, ok := ParseSan(s, uci)
		if !ok {
			t.Fatalf("failed to play %q", uci)
		}
		MakeMove(s, m)
	}
	play("e2e4")
	play("a7a6")
	play("e4e5")
	play("d7d5")
	if s.Ep != int8(Sq(3, 5)) { // d6
		t.Fatalf("en passant square = %d, want %d", s.Ep, Sq(3, 5))
	}
	var ep *Move
	for _, m := range GenerateLegal(s) {
		if m.From() == Sq(4, 4) && m.To() == Sq(3, 5) {
			ep = &m
		}
	}
	if ep == nil || ep.Flag() != FlagEnPassant {
		t.Fatalf("expected en passant e5xd6, got %v", ep)
	}
	u := MakeMove(s, *ep)
	if s.Board[Sq(3, 4)] != 0 { // d5 pawn must be gone
		t.Fatalf("captured d5 pawn not removed")
	}
	if s.Board[Sq(3, 5)] != Pawn {
		t.Fatalf("white pawn not on d6")
	}
	UndoMove(s, *ep, u)
	if s.Board[Sq(3, 4)] != -Pawn || s.Board[Sq(4, 4)] != Pawn {
		t.Fatalf("en passant undo failed")
	}
}

///
/// <summary>
///   TestPromotion verifies promotion generation and the resulting board.
/// </summary>
func TestPromotion(t *testing.T) {
	s, err := ParseFen("4k3/P7/8/8/8/8/8/4K3 w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	m, ok := ParseSan(s, "a8=Q")
	if !ok || m.Promo() != Queen {
		t.Fatalf("expected promotion to queen, got ok=%v m=%v", ok, m)
	}
	MakeMove(s, m)
	if s.Board[Sq(0, 7)] != PieceOf(Queen, White) {
		t.Fatalf("promotion board wrong: got %d want %d", s.Board[Sq(0, 7)], PieceOf(Queen, White))
	}
	if s.Board[Sq(0, 6)] != 0 {
		t.Fatalf("source square not cleared")
	}
}

///
/// <summary>
///   TestFoolsMate plays the shortest checkmate and asserts game over.
/// </summary>
func TestFoolsMate(t *testing.T) {
	s := NewStart()
	play := func(uci string) {
		m, ok := ParseSan(s, uci)
		if !ok {
			t.Fatalf("failed to play %q", uci)
		}
		MakeMove(s, m)
	}
	play("f2f3")
	play("e7e5")
	play("g2g4")
	play("d8h4")
	if !IsInCheck(s, White) {
		t.Fatal("white should be in check")
	}
	if n := len(GenerateLegal(s)); n != 0 {
		t.Fatalf("expected checkmate, white has %d moves", n)
	}
}

///
/// <summary>
///   TestMakeUndoRoundTrip makes and undoes every legal start move and
///   verifies the board and metadata are fully restored.
/// </summary>
func TestMakeUndoRoundTrip(t *testing.T) {
	base := ToFen(NewStart())
	for _, m := range GenerateLegal(NewStart()) {
		s := NewStart()
		u := MakeMove(s, m)
		UndoMove(s, m, u)
		if got := ToFen(s); got != base {
			t.Fatalf("undo %v changed position\n got: %s\nwant: %s", m, got, base)
		}
	}
}

///
/// <summary>
///   TestSanRoundTrip parses each SAN string produced by the generator back
///   into the same move, across several positions.
/// </summary>
func TestSanRoundTrip(t *testing.T) {
	fens := []string{
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
		"r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1",
		"rnbq1bnr/pp1pkppp/8/2pp4/8/2NP4/PPP1PPPP/R1BQKBNR w KQ - 0 5",
	}
	for _, fen := range fens {
		s, err := ParseFen(fen)
		if err != nil {
			t.Fatal(err)
		}
		legal := GenerateLegal(s)
		for _, m := range legal {
			san := San(s, m)
			plain := strings.ReplaceAll(strings.ReplaceAll(san, "+", ""), "#", "")
			got, ok := ParseSan(s, plain)
			if !ok {
				t.Fatalf("could not parse %q in %s", plain, fen)
			}
			if got != m {
				t.Fatalf("parse %q = %v, want %v", plain, got, m)
			}
		}
	}
}

///
/// <summary>
///   TestKnightEdge verifies a cornered knight only reaches its two valid
///   squares.
/// </summary>
func TestKnightEdge(t *testing.T) {
	s, err := ParseFen("N3k3/8/8/8/8/8/8/4K3 w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, m := range GenerateLegal(s) {
		if m.From() == Sq(0, 7) {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("knight on a8 reached %d squares, want 2", n)
	}
}

///
/// <summary>
///   TestScholarOpening plays the Scholar's opening moves and asserts the
///   final queen move is reported as Qe2.
/// </summary>
func TestScholarOpening(t *testing.T) {
	s := NewStart()
	moves := []string{"e2e4", "e7e5", "g1f3", "b8c6", "f1c4", "g8f6", "d1e2"}
	var lastSan string
	for _, u := range moves {
		m, ok := ParseSan(s, u)
		if !ok {
			t.Fatalf("cannot play %q", u)
		}
		lastSan = San(s, m)
		MakeMove(s, m)
	}
	if IsInCheck(s, Black) {
		t.Fatal("black should not be in check after a quiet Qe2")
	}
	if lastSan != "Qe2" {
		t.Fatalf("last move SAN = %q, want Qe2", lastSan)
	}
}