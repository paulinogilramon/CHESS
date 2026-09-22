///
/// Command arena plays a head-to-head match between the classical engine and
/// the neural-augmented engine (same search, different evaluation) and reports
/// the outcome percentage as a strength check.
package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"

	"chess/engine"
	"chess/nn"
)

///
/// <summary>
///   Options gathered from the command line.
/// </summary>
type options struct {
	weights string
	blend   float64
	scale   float64
	games   int
	moveMs  int
	depth   int
	openMax int
	maxPly  int
	cores   int
	seed    int64
}

///
/// <summary>
///   parseOptions reads the arena command-line flags.
/// </summary>
/// <returns>Parsed options from os.Args, or exits on a bad flag.</returns>
func parseOptions() options {
	var o options
	flag.StringVar(&o.weights, "weights", "weights.bin", "trained network weights")
	flag.Float64Var(&o.blend, "blend", 0.6, "network blend for the neural side")
	flag.Float64Var(&o.scale, "scale", 300, "centipawns per unit of network value")
	flag.IntVar(&o.games, "games", 40, "number of paired games (2 per pairing)")
	flag.IntVar(&o.moveMs, "ms", 60, "milliseconds per move for both sides")
	flag.IntVar(&o.depth, "depth", 6, "maximum search depth for both sides")
	flag.IntVar(&o.openMax, "open", 4, "maximum random opening plies")
	flag.IntVar(&o.maxPly, "maxply", 240, "game length cap (counted as a draw)")
	flag.IntVar(&o.cores, "cores", 4, "parallel worker goroutines")
	flag.Int64Var(&o.seed, "seed", 7, "opening seed")
	flag.Parse()
	return o
}

///
/// <summary>
///   outcome encodes a finished game result.
/// </summary>
type outcome int

///
/// <summary>
///   Game outcomes: win, loss and draw for the neural side.
/// </summary>
const (
	nnWin outcome = iota
	nnLoss
	draw
)

///
/// <summary>
///   key builds a compact repetition fingerprint for a position.
/// </summary>
/// <param name="s">Position to fingerprint.</param>
/// <returns>A fixed-size byte fingerprint.</returns>
func key(s *engine.State) [67]byte {
	var k [67]byte
	for i := 0; i < 64; i++ {
		k[i] = byte(s.Board[i])
	}
	k[64] = byte(s.Stm)
	k[65] = s.Castling
	k[66] = byte(s.Ep + 1)
	return k
}

///
/// <summary>
///   playOne plays a single game between two evaluation configs from an
///   opening move sequence.
/// </summary>
/// <param name="opening">Fixed random opening moves to apply first.</param>
/// <param name="cfgWhite">White's evaluation config.</param>
/// <param name="cfgBlack">Black's evaluation config.</param>
/// <param name="o">Global options.</param>
/// <returns>Whether White won, or false when the game was drawn.</returns>
func playOne(opening []engine.Move, cfgWhite, cfgBlack engine.NNConfig, o options) bool {
	var boards [2]engine.NNConfig
	boards[0], boards[1] = cfgWhite, cfgBlack
	s := engine.NewStart()
	for _, m := range opening {
		engine.MakeMove(s, m)
	}
	seen := map[[67]byte]int{}
	for ply := 0; ply < o.maxPly; ply++ {
		if s.Halfmove >= 100 {
			return false
		}
		moves := engine.GenerateLegal(s)
		if len(moves) == 0 {
			return engine.IsInCheck(s, s.Stm) && s.Stm == engine.Black
		}
		k := key(s)
		if seen[k] >= 3 {
			return false
		}
		seen[k]++
		var cfg engine.NNConfig
		if s.Stm == engine.White {
			cfg = boards[0]
		} else {
			cfg = boards[1]
		}
		m := engine.FindBestMoveWith(s, o.depth, o.moveMs, cfg)
		engine.MakeMove(s, m)
	}
	return false
}

///
/// <summary>
///   makeOpening generates a random opening move sequence from a start
///   position, shared by both games of a pairing.
/// </summary>
/// <param name="rng">Random source.</param>
/// <param name="o">Global options.</param>
/// <returns>Between zero and openMax legal moves.</returns>
func makeOpening(rng *rand.Rand, o options) []engine.Move {
	s := engine.NewStart()
	n := rng.Intn(o.openMax + 1)
	opening := make([]engine.Move, 0, n)
	for i := 0; i < n; i++ {
		moves := engine.GenerateLegal(s)
		if len(moves) == 0 {
			break
		}
		m := moves[rng.Intn(len(moves))]
		opening = append(opening, m)
		engine.MakeMove(s, m)
	}
	return opening
}

///
/// <summary>
///   matchPlayer runs a paired game with a shared opening and returns the
///   neural side's outcome for both color assignments.
/// </summary>
/// <param name="full">Full configuration for the neural side.</param>
/// <param name="rng">Generator for opening moves.</param>
/// <param name="o">Global options.</param>
/// <returns>The neural outcome per game.</returns>
func match(full engine.NNConfig, rng *rand.Rand, o options) [2]outcome {
	plain := engine.NNConfig{}
	opening := makeOpening(rng, o)
	outs := [2]outcome{nnWin, nnWin}
	whiteNn := rng.Intn(2) == 0
	outs[0] = classify(playOne(opening, configFor(whiteNn, plain, full), configFor(!whiteNn, plain, full), o), whiteNn)
	outs[1] = classify(playOne(opening, configFor(!whiteNn, plain, full), configFor(whiteNn, plain, full), o), !whiteNn)
	return outs
}

///
/// <summary>
///   configFor returns the classical or neural config for a side.
/// </summary>
/// <param name="nnSide">Whether this side uses the neural eval.</param>
/// <param name="plain">Classical config.</param>
/// <param name="full">Neural config.</param>
/// <returns>The chosen config.</returns>
func configFor(nnSide bool, plain, full engine.NNConfig) engine.NNConfig {
	if nnSide {
		return full
	}
	return plain
}

///
/// <summary>
///   classify maps a White-result and who the neural side played to an outcome.
/// </summary>
/// <param name="whiteWon">Whether White won.</param>
/// <param name="nnWhite">Whether the neural side was White.</param>
/// <returns>The neural side outcome.</returns>
func classify(whiteWon bool, nnWhite bool) outcome {
	switch {
	case whiteWon && nnWhite:
		return nnWin
	case whiteWon && !nnWhite:
		return nnLoss
	case !whiteWon && !nnWhite:
		return nnWin
	}
	return nnLoss
}

///
/// <summary>
///   tally keeps thread-safe outcome counters across workers.
/// </summary>
type tally struct {
	wins    atomic.Int64
	losses  atomic.Int64
	draws   atomic.Int64
	played  atomic.Int64
	games   atomic.Int64
}

///
/// <summary>
///   worker runs paired matches until the game cap is reached.
/// </summary>
/// <param name="cfg">Neural evaluation config.</param>
/// <param name="o">Global options.</param>
/// <param name="t">Shared tally.</param>
/// <param name="w">WaitGroup entry to signal.</param>
func worker(cfg engine.NNConfig, o options, t *tally, w *sync.WaitGroup) {
	defer w.Done()
	rng := rand.New(rand.NewSource(o.seed + t.games.Load()))
	for {
		n := t.games.Add(1)
		if n > int64(o.games) {
			return
		}
		outs := match(cfg, rng, o)
		for _, or := range outs {
			switch or {
			case nnWin:
				t.wins.Add(1)
			case nnLoss:
				t.losses.Add(1)
			default:
				t.draws.Add(1)
			}
			t.played.Add(1)
		}
	}
}

///
/// <summary>
///   main loads the network, runs the match, and prints the score.
/// </summary>
func main() {
	o := parseOptions()
	net, err := nn.Load(o.weights)
	if err != nil {
		log.Fatal(err)
	}
	cfg := engine.NNConfig{Net: net, Blend: float32(o.blend), Scale: float32(o.scale)}
	log.Printf("loaded %s (%d params)", o.weights, net.Count())
	log.Printf("match: classical vs neural blend=%.0f%% scale=%.0f, %d games, %dms/move, depth<=%d",
		o.blend*100, o.scale, o.games, o.moveMs, o.depth)

	t := &tally{}
	var wg sync.WaitGroup
	for i := 0; i < o.cores; i++ {
		wg.Add(1)
		go worker(cfg, o, t, &wg)
	}
	wg.Wait()

	played := t.played.Load()
	if played == 0 {
		log.Fatal("no games played")
	}
	fmt.Printf("played %d  wins %d  draws %d  losses %d\n",
		played, t.wins.Load(), t.draws.Load(), t.losses.Load())
	fmt.Printf("neural score: %.1f%%\n", 100*float64(t.wins.Load()+t.draws.Load()/2)/float64(played))
}