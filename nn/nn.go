///
/// Package nn implements a small three-layer feed-forward network learned by
/// supervised training on self-play positions, used to evaluate chess
/// positions. It is deliberately dependency-free so the engine can use it
/// without an import cycle.
package nn

import "math"

///
/// <summary>
///   Machine constants that define the input encoding.
/// </summary>
const (
	// FeatureCount is the number of sparse binary input features: 12 piece
	// color-type planes (own pawn..king, enemy pawn..king) times 64 squares.
	FeatureCount = 12 * Cell

	// Cell is the number of squares on the chessboard.
	Cell = 64

	// NumTypes is the number of piece types (pawn..king).
	NumTypes = 6

	// Plane is the footprint each piece color-type plane covers.
	Plane = NumTypes * Cell
)

///
/// <summary>
///   Net is a three-layer network: sparse input features to h1 (ReLU), h1 to
///   h2 (ReLU), h2 to a tanh output in (-1, 1) meaning the value for the side
///   to move.
/// </summary>
/// <param name="H1">First hidden layer width.</param>
/// <param name="H2">Second hidden layer width.</param>
/// <param name="W1">First layer weights, feature-major: [FeatureCount*H1].</param>
/// <param name="B1">First layer bias, length H1.</param>
/// <param name="W2">Second layer weights, [H1*H2].</param>
/// <param name="B2">Second layer bias, length H2.</param>
/// <param name="W3">Output layer weights, length H2.</param>
/// <param name="B3">Output bias.</param>
type Net struct {
	H1 int
	H2 int

	W1 []float32
	B1 []float32
	W2 []float32
	B2 []float32
	W3 []float32
	B3 float32
}

///
/// <summary>
///   NewNet builds a randomly initialised network using He initialisation so
///   ReLU activations keep a stable scale during training.
/// </summary>
/// <param name="h1">First hidden layer width.</param>
/// <param name="h2">Second hidden layer width.</param>
/// <param name="seed">Seed for the deterministic generator.</param>
/// <returns>A fresh randomly initialised network.</returns>
func NewNet(h1, h2 int, seed int64) *Net {
	rng := newRand(seed)
	n := &Net{H1: h1, H2: h2}
	n.W1 = make([]float32, FeatureCount*h1)
	n.B1 = make([]float32, h1)
	n.W2 = make([]float32, h1*h2)
	n.B2 = make([]float32, h2)
	n.W3 = make([]float32, h2)
	for i := range n.W1 {
		n.W1[i] = float32(rng.NormFloat64() * math.Sqrt(2/32.0))
	}
	for i := range n.W2 {
		n.W2[i] = float32(rng.NormFloat64() * math.Sqrt(2/float64(h1)))
	}
	for i := range n.W3 {
		n.W3[i] = float32(rng.NormFloat64() * math.Sqrt(2/float64(h2)))
	}
	return n
}

///
/// <summary>
///   typeOf returns the piece type magnitude of a stored piece value.
/// </summary>
/// <param name="p">Stored piece value, sign encodes color.</param>
/// <returns>One of 1..6 matching pawn..king, or zero when empty.</returns>
func typeOf(p int8) int8 {
	if p < 0 {
		return -p
	}
	return p
}

///
/// <summary>
///   EncodeFeatures renders a board and side to move into the sparse list of
///   activated input features. The board is mirrored for Black so the network
///   sees every position from the side to move's point of view.
/// </summary>
/// <param name="board">64 squares, a1=0 .. h8=63, sign encodes color.</param>
/// <param name="stm">Side to move: +1 white, -1 black.</param>
/// <returns>Sparse feature indices into the W1 row space.</returns>
func EncodeFeatures(board [64]int8, stm int8) []uint16 {
	feats := make([]uint16, 0, 32)
	for sq := 0; sq < Cell; sq++ {
		p := board[sq]
		if p == 0 {
			continue
		}
		ori := sq
		if stm < 0 {
			ori = sq ^ 56
		}
		plane := 0
		if (p > 0) != (stm > 0) {
			plane = 1
		}
		idx := (plane*NumTypes+int(typeOf(p))-1)*Cell + ori
		feats = append(feats, uint16(idx))
	}
	return feats
}

///
/// <summary>
///   Predict runs the forward pass for a set of sparse input features and
///   returns the network value from the side to move's perspective.
/// </summary>
/// <param name="feats">Sparse feature indices.</param>
/// <returns>A value in (-1, 1), positive meaning good for the side to move.</returns>
func (n *Net) Predict(feats []uint16) float32 {
	h1 := make([]float32, n.H1)
	for _, f := range feats {
		base := int(f) * n.H1
		for i := 0; i < n.H1; i++ {
			h1[i] += n.W1[base+i]
		}
	}
	for i := 0; i < n.H1; i++ {
		h1[i] += n.B1[i]
		if h1[i] < 0 {
			h1[i] = 0
		}
	}
	h2 := make([]float32, n.H2)
	for i := 0; i < n.H1; i++ {
		if h1[i] == 0 {
			continue
		}
		base := i * n.H2
		for j := 0; j < n.H2; j++ {
			h2[j] += h1[i] * n.W2[base+j]
		}
	}
	for j := 0; j < n.H2; j++ {
		h2[j] += n.B2[j]
		if h2[j] < 0 {
			h2[j] = 0
		}
	}
	z := n.B3
	for j := 0; j < n.H2; j++ {
		z += h2[j] * n.W3[j]
	}
	return float32(math.Tanh(float64(z)))
}

///
/// <summary>
///   Evaluate encodes a board and runs the forward pass, returning the side
///   to move's value in (-1, 1).
/// </summary>
/// <param name="board">64 squares, sign encodes color.</param>
/// <param name="stm">Side to move: +1 white, -1 black.</param>
/// <returns>The network value for the side to move.</returns>
func (n *Net) Evaluate(board [64]int8, stm int8) float32 {
	return n.Predict(EncodeFeatures(board, stm))
}

///
/// <summary>
///   Params returns every trainable parameter group as flat slices so the
///   trainer and serialiser can address uniform rows.
/// </summary>
/// <returns>The weights and biases in a stable order.</returns>
func (n *Net) Params() [][]float32 {
	return [][]float32{n.W1, n.B1, n.W2, n.B2, n.W3}
}

///
/// <summary>
///   Count returns the total number of trainable parameters.
/// </summary>
/// <returns>The sum of all weight and bias entries.</returns>
func (n *Net) Count() int {
	total := 1 // B3
	for _, g := range n.Params() {
		total += len(g)
	}
	return total
}