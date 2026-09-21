package engine

///
/// <summary>
///   GeneratePseudoLegal produces every pseudo-legal move in the position,
///   including castling and en passant, without yet filtering for king safety.
/// </summary>
/// <param name="s">Position to generate for.</param>
/// <returns>A slice of pseudo-legal moves.</returns>
func GeneratePseudoLegal(s *State) []Move {
	buf := make([]Move, 0, 64)
	for sq, p := range s.Board {
		if p == 0 {
			continue
		}
		if ColorOf(p) != s.Stm {
			continue
		}
		switch TypeOf(p) {
		case Pawn:
			pawnMoves(s, sq, &buf)
		case Knight:
			knightMoves(s, sq, &buf)
		case Bishop:
			slidingMoves(s, sq, diagDirs, &buf)
		case Rook:
			slidingMoves(s, sq, orthoDirs, &buf)
		case Queen:
			slidingMoves(s, sq, allDirs, &buf)
		case King:
			kingMoves(s, sq, &buf)
			castleMoves(s, sq, &buf)
		}
	}
	return buf
}

///
/// <summary>
///   GenerateLegal produces only fully legal moves: every pseudo-legal move
///   is made, checked for king safety, and undone.
/// </summary>
/// <param name="s">Position to generate for.</param>
/// <returns>A slice of legal moves.</returns>
func GenerateLegal(s *State) []Move {
	pending := GeneratePseudoLegal(s)
	legal := make([]Move, 0, len(pending))
	for _, m := range pending {
		u := MakeMove(s, m)
		mover := -s.Stm
		if !IsInCheck(s, mover) {
			legal = append(legal, m)
		}
		UndoMove(s, m, u)
	}
	return legal
}

///
/// <summary>
///   LegalDestinations answers which of the given destination squares are
///   legal from a source square, i.e. the squares a UI should show as
///   selectable.
/// </summary>
/// <param name="s">Position to generate for.</param>
/// <param name="from">Source square of interest.</param>
/// <returns>A list of legal moves from the source square.</returns>
func LegalDestinations(s *State, from int) []Move {
	out := make([]Move, 0, 8)
	for _, m := range GenerateLegal(s) {
		if m.From() == from {
			out = append(out, m)
		}
	}
	return out
}

///
/// <summary>
///   CanMove validates that a concrete move is a legal move in the position.
///   The position is never mutated.
/// </summary>
/// <param name="s">Position to validate against.</param>
/// <param name="m">Move to validate.</param>
/// <returns>True when the move is legal.</returns>
func CanMove(s *State, m Move) bool {
	for _, lm := range GenerateLegal(s) {
		if lm == m {
			return true
		}
	}
	return false
}

///
/// <summary>
///   pawnMoves appends all pseudo-legal pawn moves from a square, including
///   double pushes, captures, en passant, and promotions.
/// </summary>
/// <param name="s">Position to generate for.</param>
/// <param name="sq">Square holding the pawn.</param>
/// <param name="buf">Move accumulator.</param>
func pawnMoves(s *State, sq int, buf *[]Move) {
	c := s.Stm
	dir := 8 * int(c)
	f := SqFile(sq)
	r := SqRank(sq)
	promoRank := 7
	if c == Black {
		promoRank = 0
	}

	to := sq + dir
	if OnBoard(to) {
		if s.Board[to] == 0 {
			if SqRank(to) == promoRank {
				addPromos(sq, to, Pawn, FlagPromo, buf)
			} else {
				*buf = append(*buf, NewMove(sq, to, Pawn, 0, FlagQuiet))
				startRank := 1
				if c == Black {
					startRank = 6
				}
				if r == startRank && s.Board[sq+2*dir] == 0 {
					*buf = append(*buf, NewMove(sq, sq+2*dir, Pawn, 0, FlagDouble))
				}
			}
		}
	}

	for _, df := range []int{-1, 1} {
		tf := f + df
		if tf < 0 || tf > 7 {
			continue
		}
		to := sq + dir + df
		enemy := ColorOf(s.Board[to]) == -c
		enPassant := to >= 0 && to < 64 && s.Ep == int8(to)
		if !enemy && !enPassant {
			continue
		}
		switch {
		case SqRank(to) == promoRank:
			if enemy {
				addPromos(sq, to, Pawn, FlagPromoCapt, buf)
			} else {
				addPromos(sq, to, Pawn, FlagPromo, buf)
			}
		case enPassant:
			*buf = append(*buf, NewMove(sq, to, Pawn, 0, FlagEnPassant))
		case enemy:
			*buf = append(*buf, NewMove(sq, to, Pawn, 0, FlagQuiet))
		}
	}
}

///
/// <summary>
///   addPromos appends four promotion moves (Knight, Bishop, Rook, Queen)
///   from a source to a destination.
/// </summary>
/// <param name="from">Source square.</param>
/// <param name="to">Destination square.</param>
/// <param name="piece">Moving piece type.</param>
/// <param name="flag">Promotion flag (quiet or capture).</param>
/// <param name="buf">Move accumulator.</param>
func addPromos(from, to int, piece int8, flag byte, buf *[]Move) {
	for _, promo := range []int8{Knight, Bishop, Rook, Queen} {
		*buf = append(*buf, NewMove(from, to, piece, promo, flag))
	}
}

///
/// <summary>
///   knightMoves appends all pseudo-legal knight moves from a square.
/// </summary>
/// <param name="s">Position to generate for.</param>
/// <param name="sq">Square holding the knight.</param>
/// <param name="buf">Move accumulator.</param>
func knightMoves(s *State, sq int, buf *[]Move) {
	for _, off := range []int{17, 15, 10, 6, -6, -10, -15, -17} {
		to := sq + off
		if !knightLanding(sq, to) {
			continue
		}
		if ColorOf(s.Board[to]) == s.Stm {
			continue
		}
		*buf = append(*buf, NewMove(sq, to, Knight, 0, FlagQuiet))
	}
}

///
/// <summary>
///   slidingMoves appends all pseudo-legal sliding piece moves from a square
///   along the given directions.
/// </summary>
/// <param name="s">Position to generate for.</param>
/// <param name="sq">Square holding the piece.</param>
/// <param name="dirs">Directions to slide along.</param>
/// <param name="buf">Move accumulator.</param>
func slidingMoves(s *State, sq int, dirs []dir, buf *[]Move) {
	piece := TypeOf(s.Board[sq])
	f, r := SqFile(sq), SqRank(sq)
	for _, d := range dirs {
		nf, nr := f+d.df, r+d.dr
		for nf >= 0 && nf <= 7 && nr >= 0 && nr <= 7 {
			to := Sq(nf, nr)
			p := s.Board[to]
			if p == 0 {
				*buf = append(*buf, NewMove(sq, to, piece, 0, FlagQuiet))
			} else {
				if ColorOf(p) != s.Stm {
					*buf = append(*buf, NewMove(sq, to, piece, 0, FlagQuiet))
				}
				break
			}
			nf += d.df
			nr += d.dr
		}
	}
}

///
/// <summary>
///   kingMoves appends ordinary (non-castling) king moves from a square.
/// </summary>
/// <param name="s">Position to generate for.</param>
/// <param name="sq">Square holding the king.</param>
/// <param name="buf">Move accumulator.</param>
func kingMoves(s *State, sq int, buf *[]Move) {
	f, r := SqFile(sq), SqRank(sq)
	for _, d := range allDirs {
		nf, nr := f+d.df, r+d.dr
		if nf < 0 || nf > 7 || nr < 0 || nr > 7 {
			continue
		}
		to := Sq(nf, nr)
		if ColorOf(s.Board[to]) == s.Stm {
			continue
		}
		*buf = append(*buf, NewMove(sq, to, King, 0, FlagQuiet))
	}
}

///
/// <summary>
///   castleMoves appends castling moves for the king on its home square,
///   verifying empty paths and that no traversed square is attacked.
/// </summary>
/// <param name="s">Position to generate for.</param>
/// <param name="sq">King square (e1 for White, e8 for Black).</param>
/// <param name="buf">Move accumulator.</param>
func castleMoves(s *State, sq int, buf *[]Move) {
	if s.Stm == White {
		if s.Castling&WKS != 0 && rookPresent(s, 7) &&
			s.Board[5] == 0 && s.Board[6] == 0 &&
			!IsInCheck(s, White) &&
			!IsAttacked(s, 5, Black) && !IsAttacked(s, 6, Black) {
			*buf = append(*buf, NewMove(4, 6, King, 0, FlagCastle))
		}
		if s.Castling&WQS != 0 && rookPresent(s, 0) &&
			s.Board[1] == 0 && s.Board[2] == 0 && s.Board[3] == 0 &&
			!IsInCheck(s, White) &&
			!IsAttacked(s, 3, Black) && !IsAttacked(s, 2, Black) {
			*buf = append(*buf, NewMove(4, 2, King, 0, FlagCastle))
		}
		return
	}
	if s.Castling&BKS != 0 && rookPresent(s, 63) &&
		s.Board[61] == 0 && s.Board[62] == 0 &&
		!IsInCheck(s, Black) &&
		!IsAttacked(s, 61, White) && !IsAttacked(s, 62, White) {
		*buf = append(*buf, NewMove(60, 62, King, 0, FlagCastle))
	}
	if s.Castling&BQS != 0 && rookPresent(s, 56) &&
		s.Board[57] == 0 && s.Board[58] == 0 && s.Board[59] == 0 &&
		!IsInCheck(s, Black) &&
		!IsAttacked(s, 59, White) && !IsAttacked(s, 58, White) {
		*buf = append(*buf, NewMove(60, 58, King, 0, FlagCastle))
	}
}

///
/// <summary>
///   rookPresent confirms a friendly rook stands on the given corner square.
/// </summary>
/// <param name="s">Position to inspect.</param>
/// <param name="corner">Corner square.</param>
/// <returns>True when a rook of the side to move is on the corner.</returns>
func rookPresent(s *State, corner int) bool {
	return s.Board[corner] == PieceOf(Rook, s.Stm)
}