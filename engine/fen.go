package engine

import (
	"errors"
	"strings"
)

///
/// <summary>
///   ToFen renders the position as a FEN string.
/// </summary>
/// <param name="s">Position to serialize.</param>
/// <returns>The FEN representation.</returns>
func ToFen(s *State) string {
	var b strings.Builder
	for r := 7; r >= 0; r-- {
		empty := 0
		for f := 0; f < 8; f++ {
			p := s.Board[Sq(f, r)]
			if p == 0 {
				empty++
				continue
			}
			if empty > 0 {
				b.WriteByte(byte('0' + empty))
				empty = 0
			}
			b.WriteByte(fenPiece(p))
		}
		if empty > 0 {
			b.WriteByte(byte('0' + empty))
		}
		if r > 0 {
			b.WriteByte('/')
		}
	}

	b.WriteByte(' ')
	if s.Stm == White {
		b.WriteByte('w')
	} else {
		b.WriteByte('b')
	}

	b.WriteByte(' ')
	if s.Castling == 0 {
		b.WriteByte('-')
	} else {
		if s.Castling&WKS != 0 {
			b.WriteByte('K')
		}
		if s.Castling&WQS != 0 {
			b.WriteByte('Q')
		}
		if s.Castling&BKS != 0 {
			b.WriteByte('k')
		}
		if s.Castling&BQS != 0 {
			b.WriteByte('q')
		}
	}

	b.WriteByte(' ')
	if s.Ep < 0 {
		b.WriteByte('-')
	} else {
		b.WriteString(squareName(int(s.Ep)))
	}

	b.WriteByte(' ')
	b.WriteString(fmtInt(int(s.Halfmove)))
	b.WriteByte(' ')
	b.WriteString(fmtInt(s.Fullmove))
	return b.String()
}

///
/// <summary>
///   ParseFen builds a State from a FEN string.
/// </summary>
/// <param name="fen">Six-field FEN representation.</param>
/// <returns>The parsed position, or an error for malformed input.</returns>
func ParseFen(fen string) (*State, error) {
	parts := strings.Fields(fen)
	if len(parts) < 6 {
		return nil, errors.New("fen: expected 6 fields")
	}
	s := &State{Ep: -1}
	files := strings.Split(parts[0], "/")
	if len(files) != 8 {
		return nil, errors.New("fen: board must have 8 ranks")
	}
	for r, row := range files {
		f := 0
		for _, ch := range row {
			switch {
			case ch >= '1' && ch <= '8':
				f += int(ch - '0')
			default:
				p, ok := parsePiece(byte(ch))
				if !ok {
					return nil, errors.New("fen: unknown piece character")
				}
				if f > 7 {
					return nil, errors.New("fen: rank overflow")
				}
				s.Board[Sq(f, 7-r)] = p
				f++
			}
		}
		if f != 8 {
			return nil, errors.New("fen: rank must fill 8 files")
		}
	}

	switch parts[1] {
	case "w":
		s.Stm = White
	case "b":
		s.Stm = Black
	default:
		return nil, errors.New("fen: bad side to move")
	}

	if parts[2] != "-" {
		for _, ch := range parts[2] {
			switch ch {
			case 'K':
				s.Castling |= WKS
			case 'Q':
				s.Castling |= WQS
			case 'k':
				s.Castling |= BKS
			case 'q':
				s.Castling |= BQS
			default:
				return nil, errors.New("fen: bad castling field")
			}
		}
	}

	if parts[3] != "-" {
		sq, err := parseSquare(parts[3])
		if err != nil {
			return nil, err
		}
		s.Ep = int8(sq)
	}

	if len(parts) >= 5 {
		n, err := parseInt(parts[4])
		if err != nil || n > 200 {
			return nil, errors.New("fen: bad halfmove clock")
		}
		s.Halfmove = int8(n)
	}
	if len(parts) >= 6 {
		n, err := parseInt(parts[5])
		if err != nil {
			return nil, errors.New("fen: bad fullmove number")
		}
		s.Fullmove = n
	}
	return s, nil
}

///
/// <summary>
///   fenPiece maps a stored piece value to its FEN letter.
/// </summary>
/// <param name="p">Stored piece value.</param>
/// <returns>The FEN letter.</returns>
func fenPiece(p int8) byte {
	var ch byte
	switch TypeOf(p) {
	case Pawn:
		ch = 'p'
	case Knight:
		ch = 'n'
	case Bishop:
		ch = 'b'
	case Rook:
		ch = 'r'
	case Queen:
		ch = 'q'
	case King:
		ch = 'k'
	}
	if ColorOf(p) == White {
		return ch - 32
	}
	return ch
}

///
/// <summary>
///   parsePiece maps a FEN letter to its stored piece value.
/// </summary>
/// <param name="ch">FEN letter.</param>
/// <returns>The stored value and a success flag.</returns>
func parsePiece(ch byte) (int8, bool) {
	var typ int8
	switch ch {
	case 'P', 'p':
		typ = Pawn
	case 'N', 'n':
		typ = Knight
	case 'B', 'b':
		typ = Bishop
	case 'R', 'r':
		typ = Rook
	case 'Q', 'q':
		typ = Queen
	case 'K', 'k':
		typ = King
	default:
		return 0, false
	}
	if ch >= 'A' && ch <= 'Z' {
		return typ, true
	}
	return -typ, true
}

///
/// <summary>
///   squareName renders a square index as coordinate notation.
/// </summary>
/// <param name="sq">Square index.</param>
/// <returns>e.g. "e4".</returns>
func squareName(sq int) string {
	return string([]byte{'a' + byte(SqFile(sq)), '1' + byte(SqRank(sq))})
}

///
/// <summary>
///   parseSquare converts a coordinate square string to its index.
/// </summary>
/// <param name="name">e.g. "e4".</param>
/// <returns>The square index, or an error.</returns>
func parseSquare(name string) (int, error) {
	if len(name) != 2 {
		return 0, errors.New("fen: bad square")
	}
	f := int(name[0] - 'a')
	r := int(name[1] - '1')
	if f < 0 || f > 7 || r < 0 || r > 7 {
		return 0, errors.New("fen: bad square")
	}
	return Sq(f, r), nil
}

///
/// <summary>
///   fmtInt renders a non-negative integer as decimal text.
/// </summary>
/// <param name="n">Integer to render.</param>
/// <returns>The decimal representation.</returns>
func fmtInt(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

///
/// <summary>
///   parseInt parses a non-negative integer field from a FEN string.
/// </summary>
/// <param name="field">FEN numeric field.</param>
/// <returns>The parsed number, or an error.</returns>
func parseInt(field string) (int, error) {
	if field == "" {
		return 0, errors.New("missing number")
	}
	n := 0
	for _, ch := range field {
		if ch < '0' || ch > '9' {
			return 0, errors.New("bad number field: " + field)
		}
		n = n*10 + int(ch-'0')
	}
	return n, nil
}