///
/// <summary>
///   Exploration adds a configurable amount of nondeterminism to the root
///   move choice so the engine does not repeat identical games. When enabled,
///   the search still prefers the strongest move but picks randomly among the
///   moves within a small score margin of the best.
/// </summary>
package engine

import (
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
)

const (
	// rootMargin is the score window (centipawns) below the best move for a
	// candidate still to be considered playable.
	rootMargin = 25
)

///
/// <summary>
///   explorationBits stores the epsilon as a float64 bit pattern; zero means
///   strictly deterministic play.
/// </summary>
var explorationBits atomic.Uint64

///
/// <summary>
///   exploreMu serialises the shared generator used by the root move picker,
///   since searches may run concurrently across arena workers.
/// </summary>
var (
	exploreMu  sync.Mutex
	exploreRng = rand.New(rand.NewSource(0x5EED))
)

///
/// <summary>
///   SetExploration sets the probability (0..1) of playing a random move from
///   the near-best set instead of the single best move.
/// </summary>
/// <param name="e">Epsilon on [0,1].</param>
func SetExploration(e float64) {
	if e < 0 {
		e = 0
	}
	if e > 1 {
		e = 1
	}
	explorationBits.Store(math.Float64bits(e))
}

///
/// <summary>
///   exploration reports the current epsilon.
/// </summary>
/// <returns>The exploration probability.</returns>
func exploration() float64 {
	return math.Float64frombits(explorationBits.Load())
}

///
/// <summary>
///   rootScore records one root move and its searched value.
/// </summary>
type rootScore struct {
	m   Move
	val int
}

///
/// <summary>
///   pickRootMove returns the move to play. With no exploration it returns
///   the best move; otherwise with probability epsilon it samples uniformly
///   among the moves within margin of the best.
/// </summary>
/// <param name="scores">Every root move with its searched value.</param>
/// <param name="bestVal">Value of the best move.</param>
/// <param name="best">Best move found.</param>
/// <returns>The move to play.</returns>
func pickRootMove(scores []rootScore, bestVal int, best Move) Move {
	e := exploration()
	if e <= 0 || len(scores) < 2 {
		return best
	}
	cands := make([]Move, 0, len(scores))
	for _, rs := range scores {
		if rs.val >= bestVal-rootMargin {
			cands = append(cands, rs.m)
		}
	}
	if len(cands) < 2 {
		return best
	}
	exploreMu.Lock()
	sample := cands[exploreRng.Intn(len(cands))]
	exploreMu.Unlock()
	return sample
}