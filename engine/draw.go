package engine

///
/// <summary>
///   PositionKey returns a compact, repeatable signature of a position so the
///   threefold-repetition rule can be detected. It folds in the board, the
///   side to move, castling rights, and the en-passant square, and deliberately
///   excludes both counting clocks so the same position reached in different
///   move indexes compares equal.
/// </summary>
/// <param name="s">Position to sign.</param>
/// <returns>A stable string unique to the repeatable features.</returns>
func PositionKey(s *State) string {
	buf := make([]byte, 0, 72)
	for _, p := range s.Board {
		buf = append(buf, byte(p+16))
	}
	buf = append(buf, byte(s.Stm))
	buf = append(buf, s.Castling)
	buf = append(buf, byte(s.Ep+1))
	return string(buf)
}

///
/// <summary>
///   InsufficientMaterial reports whether neither side can possibly deliver
///   checkmate given the material left on the board, which makes the game an
///   automatic draw. It follows the standard dead-position table: bare kings;
///   a king plus a single minor against a bare king; or kings with a single
///   same-colored bishop apiece.
/// </summary>
/// <param name="s">Position to inspect.</param>
/// <returns>True when checkmate is impossible for both sides.</returns>
func InsufficientMaterial(s *State) bool {
	var majors bool
	wKnights, bKnights := 0, 0
	wBishops, bBishops := 0, 0
	var wBishopColor, bBishopColor = -1, -1
	for sq, p := range s.Board {
		if p == 0 {
			continue
		}
		switch TypeOf(p) {
		case Rook, Queen, Pawn:
			majors = true
		case Knight:
			if p > 0 {
				wKnights++
			} else {
				bKnights++
			}
		case Bishop:
			c := (SqFile(sq) + SqRank(sq)) & 1
			if p > 0 {
				wBishops++
				wBishopColor = c
			} else {
				bBishops++
				bBishopColor = c
			}
		}
	}
	if majors {
		return false
	}
	wMinors := wKnights + wBishops
	bMinors := bKnights + bBishops
	if wMinors == 0 && bMinors == 0 {
		return true
	}
	if wMinors == 1 && bMinors == 0 {
		return true
	}
	if wMinors == 0 && bMinors == 1 {
		return true
	}
	if wMinors == 1 && bMinors == 1 && wBishopColor >= 0 && bBishopColor >= 0 &&
		wBishopColor == bBishopColor {
		return true
	}
	return false
}

///
/// <summary>
///   FiftyMoveDraw reports whether the fifty-move rule applies: the halfmove
///   clock has reached 100 plies without a pawn move or capture.
/// </summary>
/// <param name="s">Position to inspect.</param>
/// <returns>True when the game is a draw by the fifty-move rule.</returns>
func FiftyMoveDraw(s *State) bool {
	return s.Halfmove >= 100
}
