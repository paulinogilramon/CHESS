package engine

///
/// <summary>
///   Undo is the fixed-size data needed to roll back a single move.
/// </summary>
/// <param name="Castling">Castling rights before the move.</param>
/// <param name="Ep">En passant square before the move.</param>
/// <param name="Halfmove">Halfmove clock before the move.</param>
/// <param name="Fullmove">Fullmove number before the move.</param>
/// <param name="Captured">Piece captured by the move, or 0.</param>
type Undo struct {
	Castling byte
	Ep       int8
	Halfmove int8
	Fullmove int
	Captured int8
}

///
/// <summary>
///   MakeMove applies an already-validated legal move to the position.
///   It performs only the required memory changes and assumes legality,
///   per the plan's separation of validation and mutation.
/// </summary>
/// <param name="s">Position to modify in place.</param>
/// <param name="m">Legal move to apply.</param>
/// <returns>The Undo record needed to reverse the move.</returns>
func MakeMove(s *State, m Move) Undo {
	from, to := m.From(), m.To()
	u := Undo{
		Castling: s.Castling,
		Ep:       s.Ep,
		Halfmove: s.Halfmove,
		Fullmove: s.Fullmove,
	}
	piece := PieceOf(m.Piece(), s.Stm)

	switch m.Flag() {
	case FlagEnPassant:
		u.Captured = PieceOf(Pawn, -s.Stm)
		s.Board[from] = 0
		s.Board[to] = piece
		s.Board[to-8*int(s.Stm)] = 0
	case FlagCastle:
		rf, rt := castlingRook(from, to)
		s.Board[from] = 0
		s.Board[to] = piece
		s.Board[rf] = 0
		s.Board[rt] = PieceOf(Rook, s.Stm)
	default:
		u.Captured = s.Board[to]
		s.Board[from] = 0
		if p := m.Promo(); p != 0 {
			s.Board[to] = PieceOf(p, s.Stm)
		} else {
			s.Board[to] = piece
		}
	}

	updateCastling(s, from, to)

	isPawn := m.Piece() == Pawn
	isCapture := m.Flag() == FlagEnPassant || u.Captured != 0
	s.Ep = -1
	if m.Flag() == FlagDouble {
		s.Ep = int8((from + to) / 2)
	}
	if isPawn || isCapture {
		s.Halfmove = 0
	} else {
		s.Halfmove++
	}
	if s.Stm == Black {
		s.Fullmove++
	}
	s.Stm = -s.Stm
	return u
}

///
/// <summary>
///   UndoMove fully reverses a move previously applied with MakeMove.
/// </summary>
/// <param name="s">Position to modify in place.</param>
/// <param name="m">The move that was applied.</param>
/// <param name="u">The Undo record captured by MakeMove.</param>
func UndoMove(s *State, m Move, u Undo) {
	from, to := m.From(), m.To()
	mover := -s.Stm

	switch m.Flag() {
	case FlagEnPassant:
		s.Board[to-8*int(mover)] = u.Captured
		s.Board[to] = 0
		s.Board[from] = PieceOf(Pawn, mover)
	case FlagCastle:
		rf, rt := castlingRook(from, to)
		s.Board[rf] = PieceOf(Rook, mover)
		s.Board[rt] = 0
		s.Board[from] = PieceOf(King, mover)
		s.Board[to] = 0
	default:
		s.Board[from] = PieceOf(m.Piece(), mover)
		s.Board[to] = u.Captured
	}

	s.Castling = u.Castling
	s.Ep = u.Ep
	s.Halfmove = u.Halfmove
	s.Fullmove = u.Fullmove
	s.Stm = mover
}

///
/// <summary>
///   castlingRook maps a castling king move to the rook's source and
///   destination squares.
/// </summary>
/// <param name="from">King source square.</param>
/// <param name="to">King destination square.</param>
/// <returns>The rook's source and destination squares.</returns>
func castlingRook(from, to int) (int, int) {
	if from == 4 {
		if to == 6 {
			return 7, 5
		}
		return 0, 3
	}
	if to == 62 {
		return 63, 61
	}
	return 56, 59
}

///
/// <summary>
///   updateCastling clears castling rights lost by the current move.
/// </summary>
/// <param name="s">Position to modify in place.</param>
/// <param name="from">Source square of the move.</param>
/// <param name="to">Destination square of the move.</param>
func updateCastling(s *State, from, to int) {
	switch from {
	case 4:
		s.Castling &^= WKS | WQS
	case 60:
		s.Castling &^= BKS | BQS
	}
	corners := []struct {
		sq  int
		bit byte
	}{
		{7, WKS}, {0, WQS}, {63, BKS}, {56, BQS},
	}
	for _, c := range corners {
		if from == c.sq || to == c.sq {
			s.Castling &^= c.bit
		}
	}
}