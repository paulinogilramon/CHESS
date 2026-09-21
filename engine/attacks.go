package engine

///
/// <summary>
///   dir describes a unit file/rank step used to walk rays and neighborhoods.
/// </summary>
type dir struct {
	df, dr int
}

///
/// <summary>
///   All eight compass directions.
/// </summary>
var allDirs = []dir{
	{1, 0}, {-1, 0}, {0, 1}, {0, -1},
	{1, 1}, {1, -1}, {-1, 1}, {-1, -1},
}

///
/// <summary>
///   The four orthogonal directions.
/// </summary>
var orthoDirs = []dir{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}

///
/// <summary>
///   The four diagonal directions.
/// </summary>
var diagDirs = []dir{{1, 1}, {1, -1}, {-1, 1}, {-1, -1}}

///
/// <summary>
///   IsAttacked reports whether a square is attacked by a given side.
///   It inspects the board directly instead of generating moves,
///   per the plan's DOP requirement.
/// </summary>
/// <param name="s">Position to inspect.</param>
/// <param name="sq">Square under attack.</param>
/// <param name="c">Attacking color.</param>
/// <returns>True when side c attacks square sq.</returns>
func IsAttacked(s *State, sq int, c Color) bool {
	r := SqRank(sq)
	f := SqFile(sq)

	// Pawn attackers: a pawn of color c sitting one rank behind, file f±1.
	pr := r - int(c)
	if pr >= 0 && pr <= 7 {
		for _, df := range []int{-1, 1} {
			pf := f + df
			if pf >= 0 && pf <= 7 && s.Board[Sq(pf, pr)] == PieceOf(Pawn, c) {
				return true
			}
		}
	}

	// Knight attackers.
	knightOffs := []int{17, 15, 10, 6, -6, -10, -15, -17}
	for _, off := range knightOffs {
		if to := sq - off; knightLanding(sq, to) && s.Board[to] == PieceOf(Knight, c) {
			return true
		}
	}

	// King attackers: any adjacent square.
	for _, d := range allDirs {
		nf, nr := f+d.df, r+d.dr
		if nf < 0 || nf > 7 || nr < 0 || nr > 7 {
			continue
		}
		if s.Board[Sq(nf, nr)] == PieceOf(King, c) {
			return true
		}
	}

	// Orthogonal rays: first piece must be a Rook or Queen of color c.
	if ray(s, sq, c, orthoDirs, Rook, Queen) {
		return true
	}

	// Diagonal rays: first piece must be a Bishop or Queen of color c.
	return ray(s, sq, c, diagDirs, Bishop, Queen)
}

///
/// <summary>
///   ray walks in each direction until it hits a piece, reporting whether
///   the first piece encountered is a piece of color c matching one of the
///   given types.
/// </summary>
/// <param name="s">Position to inspect.</param>
/// <param name="sq">Origin square.</param>
/// <param name="c">Attacking color.</param>
/// <param name="dirs">Directions to walk.</param>
/// <param name="t1">First accepted piece type.</param>
/// <param name="t2">Second accepted piece type.</param>
/// <returns>True when a matching attacking piece is found before any other blocker.</returns>
func ray(s *State, sq int, c Color, dirs []dir, t1, t2 int8) bool {
	f, r := SqFile(sq), SqRank(sq)
	for _, d := range dirs {
		nf, nr := f+d.df, r+d.dr
		for nf >= 0 && nf <= 7 && nr >= 0 && nr <= 7 {
			p := s.Board[Sq(nf, nr)]
			if p != 0 {
				if ColorOf(p) == c {
					t := TypeOf(p)
					if t == t1 || t == t2 {
						return true
					}
				}
				break
			}
			nf += d.df
			nr += d.dr
		}
	}
	return false
}

///
/// <summary>
///   IsInCheck reports whether the king of side c is attacked.
/// </summary>
/// <param name="s">Position to inspect.</param>
/// <param name="c">Side whose king is tested.</param>
/// <returns>True when c is in check.</returns>
func IsInCheck(s *State, c Color) bool {
	k := s.KingSquare(c)
	if k < 0 {
		return false
	}
	return IsAttacked(s, k, -c)
}

///
/// <summary>
///   knightLanding verifies a candidate knight jump does not wrap around
///   the board edge.
/// </summary>
/// <param name="from">Source square.</param>
/// <param name="to">Candidate target square.</param>
/// <returns>True when this is a legal knight-shaped coordinate jump.</returns>
func knightLanding(from, to int) bool {
	if !OnBoard(to) {
		return false
	}
	df := absDiff(SqFile(from), SqFile(to))
	dr := absDiff(SqRank(from), SqRank(to))
	return (df == 1 && dr == 2) || (df == 2 && dr == 1)
}

///
/// <summary>
///   absDiff returns the absolute difference of two integers.
/// </summary>
/// <param name="a">First value.</param>
/// <param name="b">Second value.</param>
/// <returns>|a-b|.</returns>
func absDiff(a, b int) int {
	if a > b {
		return a - b
	}
	return b - a
}