///
/// <summary>
///   The points system gives the engine a longer-term "motivation" beyond the
///   raw position value: moves cost points (so it wins fast), captures and
///   checks pay, promotions are rewarded by the promoted piece, and its own
///   check is punished. Point magnitudes are in centipawns so they blend
///   directly with the rest of the evaluation.
/// </summary>
package engine

import "sync/atomic"

const (
	// ptsMoveCost is paid by the mover for every move played, encouraging
	// earlier decisive results over drawn-out games.
	ptsMoveCost = 15

	// ptsCheck is earned for giving check to the opponent and lost when
	// standing in check.
	ptsCheck = 30

	// ptsPromoBonus is the extra points for promoting a pawn, scaled by the
	// promoted piece so stronger promotions pay more.
	ptsPromoQueen  = 70
	ptsPromoRook   = 50
	ptsPromoBishop = 35
	ptsPromoKnight = 30
)

///
/// <summary>
///   pointShaping gates the whole points system so the arena, GUI, and CLI
///   can run with or without it to compare behaviour.
/// </summary>
var pointShaping atomic.Bool

///
/// <summary>
///   EnablePoints turns the points system on or off for every engine search
///   running in the process.
/// </summary>
/// <param name="on">Whether to enable point shaping.</param>
func EnablePoints(on bool) {
	pointShaping.Store(on)
}

///
/// <summary>
///   PointsEnabled reports the current points system state.
/// </summary>
/// <returns>Whether point shaping is active.</returns>
func PointsEnabled() bool {
	return pointShaping.Load()
}

///
/// <summary>
///   promoPoints returns the points earned for promoting a pawn to piece
///   promo, scaled by the promoted piece.
/// </summary>
/// <param name="promo">Promoted piece type, zero for non-promotions.</param>
/// <returns>The promotion bonus points.</returns>
func promoPoints(promo int8) int {
	switch promo {
	case Queen:
		return ptsPromoQueen
	case Rook:
		return ptsPromoRook
	case Bishop:
		return ptsPromoBishop
	case Knight:
		return ptsPromoKnight
	}
	return 0
}

///
/// <summary>
///   movePoints is the net points a player earns by playing a move: the
///   promotion bonus minus the per-move cost.
/// </summary>
/// <param name="m">Move being played.</param>
/// <returns>The net movement points from the mover's perspective.</returns>
func movePoints(m Move) int {
	return promoPoints(m.Promo()) - ptsMoveCost
}

///
/// <summary>
///   checkPoints returns the check-context points from White's perspective:
///   positive when Black is in check (White gave check), negative when White
///   is in check (White is being checked).
/// </summary>
/// <param name="s">Position to inspect.</param>
/// <returns>The check points signed for White.</returns>
func checkPoints(s *State) int {
	if IsInCheck(s, White) {
		return -ptsCheck
	}
	if IsInCheck(s, Black) {
		return ptsCheck
	}
	return 0
}

///
/// <summary>
///   moveBonus is the search-adapter for movePoints with the points system
///   gate, credited to the mover after the child search returns.
/// </summary>
/// <param name="m">Move being played.</param>
/// <returns>The net movement points, or zero when shaping is off.</returns>
func (sc *searchCtx) moveBonus(m Move) int {
	if !pointShaping.Load() {
		return 0
	}
	return movePoints(m)
}

///
/// <summary>
///   MoveReward returns the points a move earns in a position before it is
///   played, signed from White's perspective: the per-move cost, the captured
///   piece value, the promotion bonus, and the check delivery bonus.
/// </summary>
/// <param name="s">Position before the move.</param>
/// <param name="m">Move about to be played.</param>
/// <returns>The move's points from White's viewpoint.</returns>
func MoveReward(s *State, m Move) int {
	reward := promoPoints(m.Promo()) - ptsMoveCost
	if m.IsCapture(s) {
		typ := Pawn
		if m.Flag() != FlagEnPassant {
			typ = TypeOf(s.Board[m.To()])
		}
		reward += pieceVal[typ]
	}
	makeMove := MakeMove(s, m)
	if IsInCheck(s, s.Stm) {
		reward += ptsCheck
	}
	UndoMove(s, m, makeMove)
	if s.Stm == Black {
		return -reward
	}
	return reward
}

///
/// <summary>
///   pointsEval adds the points-system context to a White-perspective score.
/// </summary>
/// <param name="s">Position to inspect.</param>
/// <param name="score">Score in centipawns from White's viewpoint.</param>
/// <returns>The score plus the check-context points when enabled.</returns>
func pointsEval(s *State, score int) int {
	if !pointShaping.Load() {
		return score
	}
	return score + checkPoints(s)
}