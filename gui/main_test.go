package main

import (
	"testing"

	"chess/engine"
)

///
/// <summary>
///   TestEnginePlays verifies the GUI's asynchronous engine flow: the engine
///   defaults to Black, the human moves White first, the engine replies, and
///   the reply is legal and leaves White to move again.
/// </summary>
func TestEnginePlays(t *testing.T) {
	g := newGame()
	if g.aiSide != engine.Black {
		t.Fatalf("expected engine to default to Black, got %d", g.aiSide)
	}
	if g.board.Stm != engine.White {
		t.Fatalf("expected White to move, got %d", g.board.Stm)
	}
	g.engineMove()
	if g.thinking {
		t.Fatal("engine must not move out of turn")
	}
	whiteMoves := engine.GenerateLegal(g.board)
	if len(whiteMoves) == 0 {
		t.Fatal("no white moves")
	}
	g.play(whiteMoves[0])
	if g.board.Stm != engine.Black {
		t.Fatalf("expected Black to move, got %d", g.board.Stm)
	}
	g.anim = nil
	g.engineMove()
	if !g.thinking {
		t.Fatal("engine did not start thinking on its turn")
	}
	mv := <-g.aiCh
	g.thinking = false
	if mv == 0 {
		t.Fatal("engine returned the zero move")
	}
	if !engine.CanMove(g.board, mv) {
		t.Fatalf("engine move %v not legal", mv)
	}
	g.play(mv)
	if g.board.Stm != engine.White {
		t.Fatalf("expected White to move after engine reply, got %d", g.board.Stm)
	}
}