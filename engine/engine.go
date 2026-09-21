///
/// Package engine implements a data-oriented chess move generator and
/// legality validator. Data (State) is kept separate from behavior
/// (pure functions), following the plan in Plan.md.
package engine

///
/// <summary>
///   Color identifies a side. Positive is White, negative is Black.
/// </summary>
type Color int8

///
/// <summary>
///   Side to move constants.
/// </summary>
const (
	White Color = 1
	Black Color = -1
)

///
/// <summary>
///   Piece type constants. Stored in the board as magnitude;
///   the sign of the byte encodes the color.
/// </summary>
const (
	Empty  int8 = 0
	Pawn   int8 = 1
	Knight int8 = 2
	Bishop int8 = 3
	Rook   int8 = 4
	Queen  int8 = 5
	King   int8 = 6
)

///
/// <summary>
///   Castling right bitmasks stored in State.Castling.
/// </summary>
const (
	WKS byte = 1
	WQS byte = 2
	BKS byte = 4
	BQS byte = 8
)

///
/// <summary>
///   State is the pure, contiguously stored data of a chess position.
/// </summary>
/// <param name="Board">64 squares, a1=0 .. h8=63, sign encodes color.</param>
/// <param name="Stm">Side to move.</param>
/// <param name="Castling">Castling rights bitmask.</param>
/// <param name="Ep">En passant target square, or -1.</param>
/// <param name="Halfmove">Halfmove clock for the 50-move rule.</param>
/// <param name="Fullmove">Full move number.</param>
type State struct {
	Board    [64]int8
	Stm      Color
	Castling byte
	Ep       int8
	Halfmove int8
	Fullmove int
}

///
/// <summary>
///   SqFile returns the file (0..7) of a square index.
/// </summary>
/// <param name="sq">0-based square index (a1=0 .. h8=63).</param>
/// <returns>The 0-based file (column).</returns>
func SqFile(sq int) int {
	return sq & 7
}

///
/// <summary>
///   SqRank returns the rank (0..7) of a square index.
/// </summary>
/// <param name="sq">0-based square index (a1=0 .. h8=63).</param>
/// <returns>The 0-based rank (row), 0 = rank 1.</returns>
func SqRank(sq int) int {
	return sq >> 3
}

///
/// <summary>
///   Sq builds a square index from file and rank.
/// </summary>
/// <param name="file">0-based file.</param>
/// <param name="rank">0-based rank.</param>
/// <returns>The square index.</returns>
func Sq(file, rank int) int {
	return rank*8 + file
}

///
/// <summary>
///   OnBoard reports whether a rank is within the chessboard.
/// </summary>
/// <param name="sq">Square index to test.</param>
/// <returns>True when sq is in [0,63].</returns>
func OnBoard(sq int) bool {
	return sq >= 0 && sq < 64
}

///
/// <summary>
///   ColorOf reports the color of a stored piece value.
/// </summary>
/// <param name="p">Stored piece value (sign encodes color).</param>
/// <returns>White, Black, or zero when the square is empty.</returns>
func ColorOf(p int8) Color {
	switch {
	case p > 0:
		return White
	case p < 0:
		return Black
	default:
		return 0
	}
}

///
/// <summary>
///   TypeOf reports the piece type magnitude of a stored piece value.
/// </summary>
/// <param name="p">Stored piece value (sign encodes color).</param>
/// <returns>One of Pawn..King, or Empty.</returns>
func TypeOf(p int8) int8 {
	if p < 0 {
		return -p
	}
	return p
}

///
/// <summary>
///   PieceOf builds a stored piece value from type and color.
/// </summary>
/// <param name="typ">Piece type (Pawn..King).</param>
/// <param name="c">Piece color.</param>
/// <returns>The stored board value.</returns>
func PieceOf(typ int8, c Color) int8 {
	return typ * int8(c)
}

///
/// <summary>
///   NewStart returns the standard chess starting position.
/// </summary>
/// <returns>A pointer to a fresh State with the initial position.</returns>
func NewStart() *State {
	s := &State{
		Stm:      White,
		Castling: WKS | WQS | BKS | BQS,
		Ep:       -1,
		Halfmove: 0,
		Fullmove: 1,
	}
	back := [8]int8{Rook, Knight, Bishop, Queen, King, Bishop, Knight, Rook}
	for f := 0; f < 8; f++ {
		s.Board[Sq(f, 0)] = back[f]
		s.Board[Sq(f, 1)] = Pawn
		s.Board[Sq(f, 6)] = -Pawn
		s.Board[Sq(f, 7)] = -back[f]
	}
	return s
}

///
/// <summary>
///   PieceAt returns the stored value on a square.
/// </summary>
/// <param name="sq">Square index.</param>
/// <returns>The stored piece value.</returns>
func (s *State) PieceAt(sq int) int8 {
	return s.Board[sq]
}

///
/// <summary>
///   KingSquare locates the king of a given color.
/// </summary>
/// <param name="c">Color whose king is sought.</param>
/// <returns>The king square, or -1 if the king is absent.</returns>
func (s *State) KingSquare(c Color) int {
	for sq, p := range s.Board {
		if p == King*int8(c) {
			return sq
		}
	}
	return -1
}