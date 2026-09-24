///
/// <summary>
///   Additional package documentation is in engine.go; this file adds the
///   alpha-beta searcher with quiescence used to play against the position.
/// </summary>
package engine

import (
	"sort"
	"time"
)

///
/// <summary>
///   Search score bounds. mateScore must stay well below infScore so that
///   checkmate values never overflow the alpha-beta window.
/// </summary>
const (
	infScore  = 1_000_000
	mateScore = 100_000
)

///
/// <summary>
///   pieceVal is the static material value of each piece type in centipawns,
///   indexed by the TypeOf constants.
/// </summary>
var pieceVal = [7]int{0, 100, 320, 330, 500, 900, 20000}

///
/// <summary>
///   The following piece-square tables give a positional bonus for each
///   square of a piece, from White's perspective (rank 1 is the first row of
///   the array). Black pieces read the mirror index (sq xor 56).
/// </summary>
var pstPawn = [64]int{
	0, 0, 0, 0, 0, 0, 0, 0,
	50, 50, 50, 50, 50, 50, 50, 50,
	10, 10, 20, 30, 30, 20, 10, 10,
	5, 5, 10, 25, 25, 10, 5, 5,
	0, 0, 0, 20, 20, 0, 0, 0,
	5, -5, -10, 0, 0, -10, -5, 5,
	5, 10, 10, -20, -20, 10, 10, 5,
	0, 0, 0, 0, 0, 0, 0, 0,
}

///
/// <summary>
///   Knight piece-square table.
/// </summary>
var pstKnight = [64]int{
	-50, -40, -30, -30, -30, -30, -40, -50,
	-40, -20, 0, 0, 0, 0, -20, -40,
	-30, 0, 10, 15, 15, 10, 0, -30,
	-30, 5, 15, 20, 20, 15, 5, -30,
	-30, 0, 15, 20, 20, 15, 0, -30,
	-30, 5, 10, 15, 15, 10, 5, -30,
	-40, -20, 0, 5, 5, 0, -20, -40,
	-50, -40, -30, -30, -30, -30, -40, -50,
}

///
/// <summary>
///   Bishop piece-square table.
/// </summary>
var pstBishop = [64]int{
	-20, -10, -10, -10, -10, -10, -10, -20,
	-10, 0, 0, 0, 0, 0, 0, -10,
	-10, 0, 5, 10, 10, 5, 0, -10,
	-10, 5, 5, 10, 10, 5, 5, -10,
	-10, 0, 10, 10, 10, 10, 0, -10,
	-10, 10, 10, 10, 10, 10, 10, -10,
	-10, 5, 0, 0, 0, 0, 5, -10,
	-20, -10, -10, -10, -10, -10, -10, -20,
}

///
/// <summary>
///   Rook piece-square table.
/// </summary>
var pstRook = [64]int{
	0, 0, 0, 0, 0, 0, 0, 0,
	5, 10, 10, 10, 10, 10, 10, 5,
	-5, 0, 0, 0, 0, 0, 0, -5,
	-5, 0, 0, 0, 0, 0, 0, -5,
	-5, 0, 0, 0, 0, 0, 0, -5,
	-5, 0, 0, 0, 0, 0, 0, -5,
	-5, 0, 0, 0, 0, 0, 0, -5,
	0, 0, 0, 5, 5, 0, 0, 0,
}

///
/// <summary>
///   Queen piece-square table.
/// </summary>
var pstQueen = [64]int{
	-20, -10, -10, -5, -5, -10, -10, -20,
	-10, 0, 0, 0, 0, 0, 0, -10,
	-10, 0, 5, 5, 5, 5, 0, -10,
	-5, 0, 5, 5, 5, 5, 0, -5,
	0, 0, 5, 5, 5, 5, 0, -5,
	-10, 5, 5, 5, 5, 5, 0, -10,
	-10, 0, 5, 0, 0, 0, 0, -10,
	-20, -10, -10, -5, -5, -10, -10, -20,
}

///
/// <summary>
///   King middlegame piece-square table: the home corners are the safest,
///   while the center is hostile everywhere in the early phase.
/// </summary>
var pstKing = [64]int{
	-20, -30, -30, -40, -40, -30, -30, -20,
	-20, -30, -30, -40, -40, -30, -30, -20,
	-30, -40, -40, -50, -50, -40, -40, -30,
	-30, -40, -40, -50, -50, -40, -40, -30,
	-30, -40, -40, -50, -50, -40, -40, -30,
	-40, -50, -50, -60, -60, -50, -50, -40,
	-10, -10, 0, 0, 0, 0, -10, -10,
	10, 20, 10, 0, 0, 10, 20, 10,
}

///
/// <summary>
///   King endgame piece-square table: the king becomes an active piece that
///   should move toward the center.
/// </summary>
var pstKingEnd = [64]int{
	-50, -40, -30, -20, -20, -30, -40, -50,
	-30, -20, -10, 0, 0, -10, -20, -30,
	-30, -10, 20, 30, 30, 20, -10, -30,
	-30, -10, 30, 40, 40, 30, -10, -30,
	-30, -10, 30, 40, 40, 30, -10, -30,
	-30, -10, 20, 30, 30, 20, -10, -30,
	-30, -30, 0, 0, 0, 0, -30, -30,
	-50, -30, -30, -30, -30, -30, -30, -50,
}

///
/// <summary>
///   pstByType indexes the piece-square tables by the TypeOf constants.
/// </summary>
var pstByType = [7][]int{
	{},
	pstPawn[:],
	pstKnight[:],
	pstBishop[:],
	pstRook[:],
	pstQueen[:],
	pstKing[:],
}

///
/// <summary>
///   Evaluate scores a position from White's perspective in centipawns,
///   blending the hand-crafted score with the installed neural evaluation
///   when one is active.
/// </summary>
/// <param name="s">Position to evaluate.</param>
/// <returns>The score from White's point of view.</returns>
func Evaluate(s *State) int {
	return evalWhite(s, DefaultNNConfig())
}

///
/// <summary>
///   hasBishopPair reports whether a side has at least two bishops.
/// </summary>
/// <param name="s">Position to inspect.</param>
/// <param name="c">Side to test.</param>
/// <returns>True when side c holds two or more bishops.</returns>
func hasBishopPair(s *State, c Color) bool {
	n := 0
	for sq := 0; sq < 64; sq++ {
		if s.Board[sq] == PieceOf(Bishop, c) {
			n++
		}
	}
	return n >= 2
}

///
/// <summary>
///   searchCtx carries the shared state of one search: node count, abort
///   flag for the time budget, deadline, history heuristic table, and the
///   explicit neural evaluation config when provided.
/// </summary>
type searchCtx struct {
	nodes    int
	abort    bool
	deadline time.Time
	history  [64][64]int
	nn       NNConfig
	hasNN    bool
}

///
/// <summary>
///   GenerateLegalCaptures returns only the capturing legal moves (including
///   en passant and promotion captures), used by the quiescence search.
/// </summary>
/// <param name="s">Position to generate for.</param>
/// <returns>A slice of capturing legal moves.</returns>
func GenerateLegalCaptures(s *State) []Move {
	out := make([]Move, 0, 16)
	for _, m := range GenerateLegal(s) {
		if m.IsCapture(s) {
			out = append(out, m)
		}
	}
	return out
}

///
/// <summary>
///   orderMoves sorts a slice of moves by a coarse quality estimate so that
///   the alpha-beta search examines the best candidates first.
/// </summary>
/// <param name="s">Position the moves belong to.</param>
/// <param name="moves">Moves to sort in place.</param>
func (sc *searchCtx) orderMoves(s *State, moves []Move) {
	sort.SliceStable(moves, func(i, j int) bool {
		return sc.mvvlva(s, moves[i])+sc.history[moves[i].From()][moves[i].To()] >
			sc.mvvlva(s, moves[j])+sc.history[moves[j].From()][moves[j].To()]
	})
}

///
/// <summary>
///   orderCaptures sorts captures by MVV-LVA for the quiescence search.
/// </summary>
/// <param name="s">Position the moves belong to.</param>
/// <param name="moves">Captures to sort in place.</param>
func (sc *searchCtx) orderCaptures(s *State, moves []Move) {
	sort.SliceStable(moves, func(i, j int) bool {
		return sc.mvvlva(s, moves[i]) > sc.mvvlva(s, moves[j])
	})
}

///
/// <summary>
///   mvvlva scores a move for ordering: most valuable victim first, least
///   valuable attacker, with promotions ranked hottest.
/// </summary>
/// <param name="s">Position the move belongs to.</param>
/// <param name="m">Move to score.</param>
/// <returns>A heuristic quality score.</returns>
func (sc *searchCtx) mvvlva(s *State, m Move) int {
	score := 0
	victim := m.To()
	enPassant := m.Flag() == FlagEnPassant
	occupied := s.Board[victim] != 0
	if enPassant {
		score = 10*pieceVal[Pawn] - pieceVal[Pawn]
	} else if occupied {
		score = 10*pieceVal[TypeOf(s.Board[victim])] - pieceVal[m.Piece()]
	}
	if m.Promo() != 0 {
		score += 800 + pieceVal[m.Promo()]
	}
	return score
}

///
/// <summary>
///   alphaBeta is the negamax search core with alpha-beta pruning.
/// </summary>
/// <param name="s">Position to search (mutated and restored in place).</param>
/// <param name="depth">Remaining depth in plies.</param>
/// <param name="ply">Distance from the root position.</param>
/// <param name="alpha">Lower window bound.</param>
/// <param name="beta">Upper window bound.</param>
/// <returns>The best score from the side to move's perspective.</returns>
func (sc *searchCtx) alphaBeta(s *State, depth, ply, alpha, beta int) int {
	sc.nodes++
	if sc.nodes&1023 == 0 && time.Now().After(sc.deadline) {
		sc.abort = true
		return 0
	}
	if s.Halfmove >= 100 {
		return 0
	}
	moves := GenerateLegal(s)
	if len(moves) == 0 {
		if IsInCheck(s, s.Stm) {
			return -(mateScore - ply)
		}
		return 0
	}
	if depth == 0 {
		return sc.quiesce(s, alpha, beta)
	}

	sc.orderMoves(s, moves)
	best := -infScore
	for _, m := range moves {
		u := MakeMove(s, m)
		val := -sc.alphaBeta(s, depth-1, ply+1, -beta, -alpha) + sc.moveBonus(m)
		UndoMove(s, m, u)
		if sc.abort {
			return 0
		}
		if val > best {
			best = val
			if val > alpha {
				alpha = val
				if alpha >= beta {
					sc.history[m.From()][m.To()] += depth * depth
					break
				}
			}
		}
	}
	return best
}

///
/// <summary>
///   quiesce resolves the horizon effect by continuing to examine only
///   captures after the nominal depth is reached.
/// </summary>
/// <param name="s">Position to search.</param>
/// <param name="alpha">Lower window bound.</param>
/// <param name="beta">Upper window bound.</param>
/// <returns>The quiet score from the side to move's perspective.</returns>
func (sc *searchCtx) quiesce(s *State, alpha, beta int) int {
	stand := sc.evalPos(s)
	if stand >= beta {
		return beta
	}
	if stand > alpha {
		alpha = stand
	}
	moves := GenerateLegalCaptures(s)
	sc.orderCaptures(s, moves)
	best := stand
	for _, m := range moves {
		if m.Promo() == 0 && stand+pieceVal[Queen]+200 <= alpha {
			continue
		}
		u := MakeMove(s, m)
		val := -sc.quiesce(s, -beta, -alpha) + sc.moveBonus(m)
		UndoMove(s, m, u)
		if sc.abort {
			return 0
		}
		if val > best {
			best = val
			if val > alpha {
				alpha = val
			}
			if alpha >= beta {
				break
			}
		}
	}
	return best
}

///
/// <summary>
///   SearchValue returns the engine's evaluation of a position in centipawns
///   from the side to move's perspective, using an unlimited classical
///   alpha-beta search to the requested depth. It is used to produce labels
///   for neural evaluation distillation.
/// </summary>
/// <param name="s">Position to value.</param>
/// <param name="maxDepth">Search depth in plies.</param>
/// <returns>The best achievable score for the side to move.</returns>
func SearchValue(s *State, maxDepth int) int {
	moves := GenerateLegal(s)
	if len(moves) == 0 {
		if IsInCheck(s, s.Stm) {
			return -mateScore
		}
		return 0
	}
	if s.Halfmove >= 100 {
		return 0
	}
	sc := &searchCtx{nn: NNConfig{}, hasNN: true}
	sc.deadline = time.Now().Add(365 * 24 * time.Hour)
	best := sc.evalPos(s)
	alpha, beta := -infScore, infScore
	for d := 1; d <= maxDepth; d++ {
		sc.orderMoves(s, moves)
		alpha, beta = -infScore, infScore
		for _, m := range moves {
			u := MakeMove(s, m)
			val := -sc.alphaBeta(s, d-1, 1, -beta, -alpha) + sc.moveBonus(m)
			UndoMove(s, m, u)
			if val > alpha {
				alpha = val
			}
		}
		best = alpha
	}
	return best
}

///
/// <summary>
///   FindBestMove picks the strongest move in a position using the installed
///   default evaluation (classical, or blended with a loaded neural net).
/// </summary>
/// <param name="s">Position to search.</param>
/// <param name="maxDepth">Maximum search depth in plies.</param>
/// <param name="ms">Time budget in milliseconds, or 0 for unlimited.</param>
/// <returns>The chosen move, or the zero Move when there is no legal move.</returns>
func FindBestMove(s *State, maxDepth int, ms int) Move {
	return FindBestMoveWith(s, maxDepth, ms, DefaultNNConfig())
}

///
/// <summary>
///   FindBestMoveWith picks the strongest move in a position using iterative
///   deepening with alpha-beta and an explicit evaluation configuration,
///   returning the best move of the deepest completed iteration within the
///   time budget.
/// </summary>
/// <param name="s">Position to search.</param>
/// <param name="maxDepth">Maximum search depth in plies.</param>
/// <param name="ms">Time budget in milliseconds, or 0 for unlimited.</param>
/// <param name="cfg">Neural evaluation configuration for this search only.</param>
/// <returns>The chosen move, or the zero Move when there is no legal move.</returns>
func FindBestMoveWith(s *State, maxDepth int, ms int, cfg NNConfig) Move {
	moves := GenerateLegal(s)
	if len(moves) == 0 {
		return 0
	}
	sc := &searchCtx{nn: cfg, hasNN: true}
	if ms > 0 {
		sc.deadline = time.Now().Add(time.Duration(ms) * time.Millisecond)
	} else {
		sc.deadline = time.Now().Add(365 * 24 * time.Hour)
	}
	sc.orderMoves(s, moves)

	best := moves[0]
	for d := 1; d <= maxDepth; d++ {
		perm := make([]Move, 0, len(moves))
		perm = append(perm, best)
		for _, m := range moves {
			if m != best {
				perm = append(perm, m)
			}
		}
		alpha, beta := -infScore, infScore
		curBest := perm[0]
		for _, m := range perm {
			u := MakeMove(s, m)
			val := -sc.alphaBeta(s, d-1, 1, -beta, -alpha) + sc.moveBonus(m)
			UndoMove(s, m, u)
			if sc.abort {
				return best
			}
			if val > alpha {
				alpha = val
				curBest = m
			}
		}
		best = curBest
	}
	return best
}