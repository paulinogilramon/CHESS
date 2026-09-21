package engine

///
/// <summary>
///   Move flags describe the special handling a move needs in make/undo.
/// </summary>
const (
	FlagQuiet      byte = 0
	FlagDouble     byte = 1
	FlagCastle     byte = 2
	FlagEnPassant  byte = 3
	FlagPromo      byte = 4
	FlagPromoCapt  byte = 5
)

///
/// <summary>
///   Move is a compact 32-bit integer encoding of a chess move,
///   as required by the compact representation target in Plan.md.
/// </summary>
/// <remarks>
///   Layout:
///   from  : bits 0..5, to  : bits 6..11
///   piece : bits 12..14, promo : bits 15..17
///   flag  : bits 18..20
/// </remarks>
type Move uint32

///
/// <summary>
///   NewMove packs a move from its parts.
/// </summary>
/// <param name="from">Source square.</param>
/// <param name="to">Destination square.</param>
/// <param name="piece">Moving piece type.</param>
/// <param name="promo">Promotion piece type, or 0 when not a promotion.</param>
/// <param name="flag">Move flag.</param>
/// <returns>The packed Move value.</returns>
func NewMove(from, to int, piece, promo int8, flag byte) Move {
	m := Move(from) |
		Move(to)<<6 |
		Move(piece)<<12 |
		Move(promo)<<15 |
		Move(flag)<<18
	return m
}

///
/// <summary>
///   From returns the source square of the move.
/// </summary>
/// <returns>Source square index.</returns>
func (m Move) From() int {
	return int(m & 0x3F)
}

///
/// <summary>
///   To returns the destination square of the move.
/// </summary>
/// <returns>Destination square index.</returns>
func (m Move) To() int {
	return int((m >> 6) & 0x3F)
}

///
/// <summary>
///   Piece returns the moving piece type.
/// </summary>
/// <returns>Piece type (Pawn..King).</returns>
func (m Move) Piece() int8 {
	return int8((m >> 12) & 0x7)
}

///
/// <summary>
///   Promo returns the promotion piece type, or 0 when not a promotion.
/// </summary>
/// <returns>Promotion piece type.</returns>
func (m Move) Promo() int8 {
	return int8((m >> 15) & 0x7)
}

///
/// <summary>
///   Flag returns the move's special-handling flag.
/// </summary>
/// <returns>The move flag.</returns>
func (m Move) Flag() byte {
	return byte((m >> 18) & 0x7)
}

///
/// <summary>
///   IsCapture reports whether the move captures a piece,
///   either via an occupied destination or en passant.
/// </summary>
/// <param name="s">Position the move is applied in.</param>
/// <returns>True when the move is a capture.</returns>
func (m Move) IsCapture(s *State) bool {
	if m.Flag() == FlagEnPassant || m.Flag() == FlagPromoCapt {
		return true
	}
	return s.Board[m.To()] != 0
}

///
/// <summary>
///   String renders the move in coordinate (UCI) notation.
/// </summary>
/// <returns>e.g. "e2e4", "e7e8q".</returns>
func (m Move) String() string {
	sq := func(n int) string {
		return string([]byte{'a' + byte(SqFile(n)), '1' + byte(SqRank(n))})
	}
	out := sq(m.From()) + sq(m.To())
	if p := m.Promo(); p != 0 {
		out += string(promoChar(p))
	}
	return out
}

///
/// <summary>
///   promoChar maps a piece type to its lowercase coordinate letter.
/// </summary>
/// <param name="typ">Piece type.</param>
/// <returns>Single character name of the piece.</returns>
func promoChar(typ int8) byte {
	switch typ {
	case Knight:
		return 'n'
	case Bishop:
		return 'b'
	case Rook:
		return 'r'
	case Queen:
		return 'q'
	}
	return '?'
}