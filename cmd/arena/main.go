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
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"chess/engine"
	"chess/nn"
)

///
/// <summary>
///   Options gathered from the command line.
/// </summary>
type options struct {
	weights  string
	weights2 string
	blend    float64
	scale    float64
	games    int
	moveMs   int
	depth    int
	openMax  int
	maxPly   int
	cores    int
	seed     int64
	san      bool
	points   bool
	explore  float64
	labelA   string
	labelB   string
}

///
/// <summary>
///   parseOptions reads the arena command-line flags.
/// </summary>
/// <returns>Parsed options from os.Args, or exits on a bad flag.</returns>
func parseOptions() options {
	var o options
	flag.StringVar(&o.weights, "weights", "weights.bin", "trained network weights")
	flag.StringVar(&o.weights2, "weights2", "", "second weights file; when set plays these against -weights head to head")
	flag.Float64Var(&o.blend, "blend", 0.6, "network blend for the neural side")
	flag.Float64Var(&o.scale, "scale", 300, "centipawns per unit of network value")
	flag.IntVar(&o.games, "games", 40, "number of paired games (2 per pairing)")
	flag.IntVar(&o.moveMs, "ms", 60, "milliseconds per move for both sides")
	flag.IntVar(&o.depth, "depth", 6, "maximum search depth for both sides")
	flag.IntVar(&o.openMax, "open", 4, "maximum random opening plies")
	flag.IntVar(&o.maxPly, "maxply", 240, "game length cap (counted as a draw)")
	flag.IntVar(&o.cores, "cores", 4, "parallel worker goroutines")
	flag.Int64Var(&o.seed, "seed", 7, "opening seed")
	flag.BoolVar(&o.san, "san", false, "print each move live (use cores=1, games=1)")
	flag.BoolVar(&o.points, "points", false, "enable the move/capture/promotion/check points system")
	flag.Float64Var(&o.explore, "explore", 0, "epsilon-greedy variety among near-best moves")
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
			if o.san {
				fmt.Println("  (50-move rule)")
			}
			return false
		}
		moves := engine.GenerateLegal(s)
		if len(moves) == 0 {
			mated := engine.IsInCheck(s, s.Stm) && s.Stm == engine.Black
			if o.san {
				if engine.IsInCheck(s, s.Stm) {
					fmt.Println("  #")   
				} else {
					fmt.Println("  (stalemate)")
				}
			}
			return mated
		}
		k := key(s)
		if seen[k] >= 3 {
			if o.san {
				fmt.Println("  (repetition)")
			}
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
		san := engine.San(s, m)
		mover := s.Stm
		fullmove := s.Fullmove
		engine.MakeMove(s, m)
		if o.san {
			if mover == engine.White {
				fmt.Printf("%d. %s  ", fullmove, san)
			} else {
				fmt.Printf("%d... %s\n", fullmove, san)
			}
			time.Sleep(70 * time.Millisecond)
		}
	}
	if o.san {
		fmt.Println("  (chart cap, draw)")
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
///   first side's outcome for both color assignments.
/// </summary>
/// <param name="a">Configuration for side A (the reported side).</param>
/// <param name="b">Configuration for side B.</param>
/// <param name="rng">Generator for opening moves.</param>
/// <param name="o">Global options.</param>
/// <returns>The side A outcome per game.</returns>
func match(a, b engine.NNConfig, rng *rand.Rand, o options) [2]outcome {
	opening := makeOpening(rng, o)
	outs := [2]outcome{nnWin, nnWin}
	whiteA := rng.Intn(2) == 0
	if o.san {
		white, black := configFor(whiteA, a, b), configFor(!whiteA, a, b)
		fmt.Printf("\n=== %s (White) vs %s (Black) ===\n", labelFor(white, a, b, o), labelFor(black, a, b, o))
	}
	outs[0] = classify(playOne(opening, configFor(whiteA, a, b), configFor(!whiteA, a, b), o), whiteA)
	outs[1] = classify(playOne(opening, configFor(!whiteA, a, b), configFor(whiteA, a, b), o), !whiteA)
	return outs
}

///
/// <summary>
///   labelFor returns which labeled engine a config belongs to.
/// </summary>
/// <param name="c">Config to identify.</param>
/// <param name="a">Side A config.</param>
/// <param name="b">Side B config.</param>
/// <param name="o">Global options holding the labels.</param>
/// <returns>The label of the matching side.</returns>
func labelFor(c engine.NNConfig, a, b engine.NNConfig, o options) string {
	if c == a {
		return o.labelA
	}
	return o.labelB
}

///
/// <summary>
///   configFor returns one of two evaluation configs for a side.
/// </summary>
/// <param name="sideA">Whether this side plays with config a.</param>
/// <param name="a">Config a used when sideA is true.</param>
/// <param name="b">Config b used when sideA is false.</param>
/// <returns>The chosen config.</returns>
func configFor(sideA bool, a, b engine.NNConfig) engine.NNConfig {
	if sideA {
		return a
	}
	return b
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
/// <param name="a">Side A evaluation config.</param>
/// <param name="b">Side B evaluation config.</param>
/// <param name="o">Global options.</param>
/// <param name="t">Shared tally.</param>
/// <param name="w">WaitGroup entry to signal.</param>
func worker(a, b engine.NNConfig, o options, t *tally, w *sync.WaitGroup) {
	defer w.Done()
	rng := rand.New(rand.NewSource(o.seed + t.games.Load()))
	for {
		n := t.games.Add(1)
		if n > int64(o.games) {
			return
		}
		outs := match(a, b, rng, o)
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
	mkCfg := func(w string) (engine.NNConfig, error) {
		n, err := nn.Load(w)
		if err != nil {
			return engine.NNConfig{}, err
		}
		return engine.NNConfig{Net: n, Blend: float32(o.blend), Scale: float32(o.scale)}, nil
	}
	cfgA := engine.NNConfig{Net: net, Blend: float32(o.blend), Scale: float32(o.scale)}
	cfgB := engine.NNConfig{}
	label := "classical"
	o.labelA = filepath.Base(o.weights)
	o.labelB = label
	if o.weights2 != "" {
		cfgB, err = mkCfg(o.weights2)
		if err != nil {
			log.Fatal(err)
		}
		label = filepath.Base(o.weights2)
		o.labelB = label
	}
	log.Printf("loaded %s (%d params)", o.weights, net.Count())
	log.Printf("match: %s(B%d%%/S%.0f) vs %s(B%d%%/S%.0f), %d games, %dms/move, depth<=%d",
		o.weights, int(o.blend*100), o.scale, label, int(o.blend*100), o.scale, o.games, o.moveMs, o.depth)

	t := &tally{}
	if o.points {
		engine.EnablePoints(true)
		log.Println("points system on")
	}
	if o.explore > 0 {
		engine.SetExploration(o.explore)
		log.Printf("exploration on (%.0f%%)", o.explore*100)
	}
	var wg sync.WaitGroup
	for i := 0; i < o.cores; i++ {
		wg.Add(1)
		go worker(cfgA, cfgB, o, t, &wg)
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