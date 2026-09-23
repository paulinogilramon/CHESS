package engine

import (
	"testing"
)

///
/// <summary>
///   TestFindBestMoveChoosesFoolsMate verifies the depth-4 search, with the
///   classical evaluator (no NN file expected in the package dir), recognises
///   that Qh4 delivers mate in one and therefore runs the mating move first.
/// </summary>
func TestFindBestMoveChoosesFoolsMate(t *testing.T) {
	s, err := ParseFen("rnbqkbnr/pppp1ppp/8/4p3/6P1/5P2/PPPPP2P/RNBQKBNR b KQkq - 1 2")
	if err != nil {
		t.Fatal(err)
	}
	m := FindBestMove(s, 5, 30000)
	if San(s, m) != "Qh4#" {
		t.Fatalf("FindBestMove(%d) chose %s, want Qh4#", 5, San(s, m))
	}
}