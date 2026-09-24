package engine

import (
	"math"
	"sync/atomic"

	"chess/nn"
)

///
/// <summary>
///   NNConfig carries the learned evaluation parameters for one search:
///   an optional network and the blend/scale used to combine it with the
///   classical material + positional score.
/// </summary>
/// <param name="Net">Trained value network, or nil to keep classical eval.</param>
/// <param name="Blend">Weight of the network score on [0,1]; 0 keeps classical.</param>
/// <param name="Scale">Centipawn gain applied per unit of network output.</param>
type NNConfig struct {
	Net   *nn.Net
	Blend float32
	Scale float32
}

///
/// <summary>
///   defaultNN is the process-wide neural evaluation used by FindBestMove
///   and Evaluate when no explicit config is passed. It is read with atomics
///   so parallel arenas never race against it.
/// </summary>
var defaultNN atomic.Pointer[NNConfig]

///
/// <summary>
///   SetDefaultNN installs the neural evaluation used by the CLI, GUI, and
///   plain FindBestMove. Pass a nil-config copy of zero value to revert to
///   purely classical evaluation.
/// </summary>
/// <param name="cfg">Evaluation configuration to use.</param>
func SetDefaultNN(cfg NNConfig) {
	if cfg.Net == nil {
		defaultNN.Store(nil)
		return
	}
	c := cfg
	defaultNN.Store(&c)
}

///
/// <summary>
///   LoadNN loads a network from a weights file and activates it as the
///   default evaluation with the given blend and scale.
/// </summary>
/// <param name="path">Binary weights path.</param>
/// <param name="blend">Blend weight on [0,1].</param>
/// <param name="scale">Centipawns per unit of network value.</param>
/// <returns>The loaded network, or an error.</returns>
func LoadNN(path string, blend, scale float32) (*nn.Net, error) {
	n, err := nn.Load(path)
	if err != nil {
		return nil, err
	}
	SetDefaultNN(NNConfig{Net: n, Blend: blend, Scale: scale})
	return n, nil
}

///
/// <summary>
///   DefaultNNConfig returns the currently installed default evaluation.
/// </summary>
/// <returns>The active config, or a zero value when classical.</returns>
func DefaultNNConfig() NNConfig {
	d := defaultNN.Load()
	if d == nil {
		return NNConfig{}
	}
	return *d
}

///
/// <summary>
///   classicalScore computes the hand-crafted material, piece-square, bishop
///   pair and tempo evaluation from White's perspective in centipawns.
/// </summary>
/// <param name="s">Position to score.</param>
/// <returns>The classical score from White's viewpoint.</returns>
func classicalScore(s *State) int {
	score := 0
	phase := 0
	for sq := 0; sq < 64; sq++ {
		p := s.Board[sq]
		if p == 0 {
			continue
		}
		typ := TypeOf(p)
		idx := sq
		if p < 0 {
			idx = sq ^ 56
		}
		val := pieceVal[typ]
		switch typ {
		case Knight, Bishop:
			phase++
		case Rook:
			phase += 2
		case Queen:
			phase += 4
		}
		if typ == King {
			val += (pstKing[idx]*phase + pstKingEnd[idx]*(24-phase)) / 24
		} else {
			val += pstByType[typ][idx]
		}
		if p > 0 {
			score += val
		} else {
			score -= val
		}
	}
	if hasBishopPair(s, White) {
		score += 30
	}
	if hasBishopPair(s, Black) {
		score -= 30
	}
	if s.Stm == White {
		score += 15
	} else {
		score -= 15
	}
	return score
}

///
/// <summary>
///   evalWhite blends the classical score with the learned network value when
///   a network is configured, expressed from White's perspective.
/// </summary>
/// <param name="s">Position to score.</param>
/// <param name="cfg">Neural evaluation configuration.</param>
/// <returns>The blended score from White's viewpoint in centipawns.</returns>
func evalWhite(s *State, cfg NNConfig) int {
	cl := classicalScore(s)
	if cfg.Net == nil {
		return pointsEval(s, cl)
	}
	v := cfg.Net.Evaluate(s.Board, int8(s.Stm))
	if s.Stm == Black {
		v = -v
	}
	nnCp := float64(cfg.Scale) * float64(v)
	blend := float64(cfg.Blend)
	return pointsEval(s, int(math.Round(float64(cl)*(1-blend)+blend*nnCp)))
}

///
/// <summary>
///   evalPos scores a position from the side to move's perspective, using the
///   searcher's explicit config or falling back to the process default.
/// </summary>
/// <param name="s">Position to score.</param>
/// <returns>The score from the side to move's viewpoint.</returns>
func (sc *searchCtx) evalPos(s *State) int {
	cfg := sc.nn
	if !sc.hasNN {
		cfg = DefaultNNConfig()
	}
	sc_ := evalWhite(s, cfg)
	if s.Stm == Black {
		return -sc_
	}
	return sc_
}