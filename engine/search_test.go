package engine

import (
	"testing"
	"time"
)

///
/// <summary>
///   TestEvaluateStart verifies the symmetric starting position evaluates to
///   near zero, preserving only the small side-to-move tempo bonus.
/// </summary>
/// <param name="t">Test context.</param>
func TestEvaluateStart(t *testing.T) {
	s := NewStart()
	if sc := Evaluate(s); sc < -20 || sc > 40 {
		t.Fatalf("start score = %d, want small tempo-centric value", sc)
	}
}

///
/// <summary>
///   TestMateInOne finds a forced queen mate from a back-rank position.
/// </summary>
/// <param name="t">Test context.</param>
func TestMateInOne(t *testing.T) {
	s, err := ParseFen("6k1/5ppp/8/8/8/8/8/1R4K1 w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	mv := FindBestMove(s, 3, 0)
	if mv == 0 {
		t.Fatal("no move found")
	}
	if mv.To() != Sq(1, 7) { // Rb1-b8
		t.Fatalf("expected back-rank mate to b8, got %v", mv)
	}
	guards := CanMove(s, mv)
	u := MakeMove(s, mv)
	mate := IsInCheck(s, Black) && len(GenerateLegal(s)) == 0
	UndoMove(s, mv, u)
	if !guards || !mate {
		t.Fatalf("search chose %v, want a legal move that checkmates", mv)
	}
}

///
/// <summary>
///   TestFindCapture confirms the engine takes an undefended queen rather
///   than playing a quiet move.
/// </summary>
/// <param name="t">Test context.</param>
func TestFindCapture(t *testing.T) {
	s, err := ParseFen("6k1/8/8/q7/8/8/8/R6K w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	mv := FindBestMove(s, 2, 1000)
	if mv == 0 || mv.From() != 0 || mv.To() != 32 { // Ra1xa5
		t.Fatalf("expected Ra1xa5, got %v", mv)
	}
	if !mv.IsCapture(s) {
		t.Fatal("chosen move is not the capture")
	}
}

///
/// <summary>
///   TestSearchMiddlegame runs a time-bounded search on an opening position
///   and verifies the picked move is legal and responsive.
/// </summary>
/// <param name="t">Test context.</param>
func TestSearchMiddlegame(t *testing.T) {
	s, err := ParseFen("r1bqkbnr/pppp1ppp/2n5/4p3/4P3/5N2/PPPP1PPP/RNBQKB1R w KQkq - 2 3")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	mv := FindBestMove(s, 4, 3000)
	elapsed := time.Since(start)
	if mv == 0 {
		t.Fatal("no move found")
	}
	if !CanMove(s, mv) {
		t.Fatalf("search chose illegal move %v", mv)
	}
	if elapsed > 30*time.Second {
		t.Fatalf("search exceeded budget (%.1fs)", elapsed.Seconds())
	}
}

///
/// <summary>
///   TestSearchDeterministic asserts the same position yields the same move
///   on repeat runs at fixed depth, i.e. the search is deterministic.
/// </summary>
/// <param name="t">Test context.</param>
func TestSearchDeterministic(t *testing.T) {
	s := NewStart()
	a := FindBestMove(s, 3, 0)
	b := FindBestMove(s, 3, 0)
	if a != b {
		t.Fatalf("non-deterministic search: %v vs %v", a, b)
	}
}

///
/// <summary>
///   TestSearchStability ensures repeats at different depths return moves the
///   engine itself considers legal.
/// </summary>
/// <param name="t">Test context.</param>
func TestSearchStability(t *testing.T) {
	s := NewStart()
	if mv := FindBestMove(s, 1, 0); !CanMove(s, mv) {
		t.Fatal("depth-1 search returned illegal move")
	}
	if mv := FindBestMove(s, 3, 0); !CanMove(s, mv) {
		t.Fatal("depth-3 search returned illegal move")
	}
}