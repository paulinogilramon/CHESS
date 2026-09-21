package engine

import (
	"strings"
)

///
/// <summary>
///   San renders a legal move in Standard Algebraic Notation, including
///   check and mate suffixes when applicable.
/// </summary>
/// <param name="s">Position (side to move is the moving side).</param>
/// <param name="m">Legal move to render.</param>
/// <returns>The SAN text, e.g. "Nf3", "exd5", "O-O", "e8=Q#".</returns>
func San(s *State, m Move) string {
	base := sanBase(s, m)
	if !CanMove(s, m) {
		return base
	}
	u := MakeMove(s, m)
	if len(GenerateLegal(s)) == 0 {
		UndoMove(s, m, u)
		return base + "#"
	}
	if IsInCheck(s, -s.Stm) {
		UndoMove(s, m, u)
		return base + "+"
	}
	UndoMove(s, m, u)
	return base
}

///
/// <summary>
///   sanBase renders a move in SAN without check/mate suffixes.
/// </summary>
/// <param name="s">Position (side to move is the moving side).</param>
/// <param name="m">Legal move to render.</param>
/// <returns>The SAN text without suffix.</returns>
func sanBase(s *State, m Move) string {
	switch m.Flag() {
	case FlagCastle:
		if m.To() == 6 || m.To() == 62 {
			return "O-O"
		}
		return "O-O-O"
	}

	to := squareName(m.To())
	if m.Piece() == Pawn {
		base := ""
		if m.IsCapture(s) {
			base = string(byte('a' + SqFile(m.From()))) + "x"
		}
		base += to
		if p := m.Promo(); p != 0 {
			base += "=" + pieceName(p)
		}
		return base
	}

	out := pieceName(m.Piece())
	amb := disambiguation(s, m)
	out += amb
	if m.IsCapture(s) {
		out += "x"
	}
	out += to
	if p := m.Promo(); p != 0 {
		out += "=" + pieceName(p)
	}
	return out
}

///
/// <summary>
///   rivalMoves lists legal moves of the same piece type landing on a square.
/// </summary>
/// <param name="s">Position.</param>
/// <param name="typ">Piece type to consider.</param>
/// <param name="to">Destination square.</param>
/// <returns>Legal source square indices.</returns>
func rivalMoves(s *State, typ int8, to int) []int {
	src := make([]int, 0, 4)
	for _, lm := range GenerateLegal(s) {
		if lm.Piece() == typ && lm.To() == to {
			src = append(src, lm.From())
		}
	}
	return src
}

///
/// <summary>
///   disambiguation computes the SAN source disambiguation prefix for a
///   piece move, choosing file, rank, or both when needed.
/// </summary>
/// <param name="s">Position.</param>
/// <param name="m">Move to disambiguate.</param>
/// <returns>The disambiguation prefix, or an empty string.</returns>
func disambiguation(s *State, m Move) string {
	others := make([]int, 0, 3)
	for _, from := range rivalMoves(s, m.Piece(), m.To()) {
		if from != m.From() {
			others = append(others, from)
		}
	}
	if len(others) == 0 {
		return ""
	}
	sameFile, sameRank := false, false
	for _, from := range others {
		if SqFile(from) == SqFile(m.From()) {
			sameFile = true
		}
		if SqRank(from) == SqRank(m.From()) {
			sameRank = true
		}
	}
	switch {
	case !sameFile:
		return string(byte('a' + SqFile(m.From())))
	case !sameRank:
		return string(byte('1' + SqRank(m.From())))
	default:
		return string([]byte{byte('a' + SqFile(m.From())), byte('1' + SqRank(m.From()))})
	}
}

///
/// <summary>
///   pieceName returns the upper-case SAN letter for a piece type.
/// </summary>
/// <param name="typ">Piece type.</param>
/// <returns>The SAN letter, or an empty string for pawns.</returns>
func pieceName(typ int8) string {
	switch typ {
	case Knight:
		return "N"
	case Bishop:
		return "B"
	case Rook:
		return "R"
	case Queen:
		return "Q"
	case King:
		return "K"
	}
	return ""
}

///
/// <summary>
///   ParseSan resolves textual input into a legal move in the position.
///   Accepts SAN (e.g. "Nf3", "exd5", "O-O", "e8=Q") and coordinate
///   notation (e.g. "e2e4", "e2e4q", "e2-e4").
/// </summary>
/// <param name="s">Position to apply the move in.</param>
/// <param name="text">User-provided move text.</param>
/// <returns>The matched legal move and true, or a zero move and false.</returns>
func ParseSan(s *State, text string) (Move, bool) {
	t := strings.TrimSpace(text)
	if t == "" {
		return 0, false
	}
	if m, ok := parseCoordinate(s, t); ok {
		return m, true
	}

	want := normalizeSAN(t)
	for _, lm := range GenerateLegal(s) {
		if normalizeSAN(sanBase(s, lm)) == want {
			return lm, true
		}
	}
	return 0, false
}

///
/// <summary>
///   normalizeSAN strips check, mate and annotation characters so that
///   user input and generated text match.
/// </summary>
/// <param name="text">SAN text possibly carrying suffixes.</param>
/// <returns>Cleaned SAN text.</returns>
func normalizeSAN(text string) string {
	t := strings.ToUpper(text)
	t = strings.ReplaceAll(t, "EP", "")
	t = strings.ReplaceAll(t, "E.P.", "")
	for _, r := range "+#!?" {
		t = strings.ReplaceAll(t, string(r), "")
	}
	t = strings.ReplaceAll(t, "0-0-0", "O-O-O")
	t = strings.ReplaceAll(t, "0-0", "O-O")
	t = strings.ReplaceAll(t, "00", "O-O")
	return t
}

///
/// <summary>
///   parseCoordinate matches coordinate (UCI-style) input.
/// </summary>
/// <param name="s">Position to apply the move in.</param>
/// <param name="text">Input text.</param>
/// <returns>A valid move and true, or zero and false.</returns>
func parseCoordinate(s *State, text string) (Move, bool) {
	t := strings.ReplaceAll(text, "-", "")
	if len(t) < 4 {
		return 0, false
	}
	from, err1 := parseSquare(t[:2])
	to, err2 := parseSquare(t[2:4])
	if err1 != nil || err2 != nil || !OnBoard(from) || !OnBoard(to) {
		return 0, false
	}
	var promo int8
	if len(t) > 4 {
		switch t[4] {
		case 'N', 'n':
			promo = Knight
		case 'B', 'b':
			promo = Bishop
		case 'R', 'r':
			promo = Rook
		case 'Q', 'q':
			promo = Queen
		default:
			return 0, false
		}
	}
	for _, lm := range GenerateLegal(s) {
		if lm.From() == from && lm.To() == to && lm.Promo() == promo {
			return lm, true
		}
	}
	return 0, false
}