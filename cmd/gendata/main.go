///
/// Command gendata runs self-play games with the classical engine and writes
/// every position paired with the final game result (from White's
/// perspective) as a binary training dataset.
package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
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
	games   int
	cores   int
	depth   int
	openMax int
	maxPly  int
	out     string
	mirror  bool
	noise   float64
	novelty float64
	pos     int64
	seed    int64
}

///
/// <summary>
///   parseOptions reads the gendata command-line flags.
/// </summary>
/// <returns>Parsed options from os.Args, or exits on a bad flag.</returns>
func parseOptions() options {
	var o options
	flag.IntVar(&o.games, "games", 2000, "number of self-play games")
	flag.IntVar(&o.cores, "cores", 4, "parallel worker goroutines")
	flag.IntVar(&o.depth, "depth", 3, "search depth per move")
	flag.IntVar(&o.openMax, "open", 4, "maximum random opening plies before searching")
	flag.IntVar(&o.maxPly, "maxply", 160, "game length cap (discards the game)")
	flag.StringVar(&o.out, "out", "dataset.bin", "output dataset path")
	flag.BoolVar(&o.mirror, "mirror", true, "also emit the color-mirrored position")
	flag.Float64Var(&o.noise, "noise", 0.2, "probability of playing a random opening move")
	flag.Float64Var(&o.novelty, "novelty", 1.0, "probability of playing the least-visited legal move")
	flag.Int64Var(&o.pos, "pos", 0, "minimum positions to emit before stopping (0 = use games)")
	flag.Int64Var(&o.seed, "seed", 1, "worker seed offset")
	flag.Parse()
	return o
}

///
/// <summary>
///   posKey builds a compact repetition key: board, side to move, castling
///   rights and en passant square.
/// </summary>
/// <param name="s">Position to fingerprint.</param>
/// <returns>A fixed-size byte fingerprint.</returns>
func posKey(s *engine.State) [67]byte {
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
///   mirrorBoard returns the color-flipped and rank-mirrored copy of a board,
///   used to double the dataset symmetrically.
/// </summary>
/// <param name="b">Source board.</param>
/// <returns>The mirrored board.</returns>
func mirrorBoard(b [64]int8) [64]int8 {
	var m [64]int8
	for sq := 0; sq < 64; sq++ {
		m[sq^56] = -b[sq]
	}
	return m
}

///
/// <summary>
///   seenPos counts how many self-play positions have been visited across all
///   workers so the novelty selector can force under-represented lines.
/// </summary>
type seenPos struct {
	mu     sync.Mutex
	counts map[[67]byte]int
}

///
/// <summary>
///   newSeenPos returns an empty shared position counter.
/// </summary>
/// <returns>A zeroed counter with an allocated map.</returns>
func newSeenPos() *seenPos {
	return &seenPos{counts: make(map[[67]byte]int)}
}

///
/// <summary>
///   add records one more visit to a position key.
/// </summary>
/// <param name="k">Position fingerprint.</param>
func (p *seenPos) add(k [67]byte) {
	p.mu.Lock()
	p.counts[k]++
	p.mu.Unlock()
}

///
/// <summary>
///   count returns how often a position key has been visited so far.
/// </summary>
/// <param name="k">Position fingerprint.</param>
/// <returns>The visit count.</returns>
func (p *seenPos) count(k [67]byte) int {
	p.mu.Lock()
	c := p.counts[k]
	p.mu.Unlock()
	return c
}

///
/// <summary>
///   pickDiverse forces novelty: from the legal moves it picks the one whose
///   resulting position has been seen the fewest times, breaking ties at
///   random, so every iteration plays distinct moves.
/// </summary>
/// <param name="s">Position to move in.</param>
/// <param name="moves">Legal moves.</param>
/// <param name="global">Shared position counter.</param>
/// <param name="rng">Random source for tie-breaking.</param>
/// <returns>The least-visited legal move.</returns>
func pickDiverse(s *engine.State, moves []engine.Move, global *seenPos, rng *rand.Rand) engine.Move {
	bestCount := int(^uint(0) >> 1)
	cands := make([]engine.Move, 0, len(moves))
	for _, m := range moves {
		u := engine.MakeMove(s, m)
		c := global.count(posKey(s))
		engine.UndoMove(s, m, u)
		if c < bestCount {
			bestCount = c
			cands = cands[:1]
			cands[0] = m
		} else if c == bestCount {
			cands = append(cands, m)
		}
	}
	return cands[rng.Intn(len(cands))]
}

///
/// <summary>
///   playGame plays one self-play game and returns every position paired with
///   the final result from White's perspective. Games hitting the ply cap are
///   discarded to avoid mislabeled truncated positions.
/// </summary>
/// <param name="rng">Random source for the opening.</param>
/// <param name="o">Global options.</param>
/// <param name="global">Shared position counter driving novelty.</param>
/// <returns>The collected positions, the White result, and whether to discard.</returns>
func playGame(rng *rand.Rand, o options, global *seenPos) ([]nn.RawSample, float32, bool) {
	s := engine.NewStart()
	moves := engine.GenerateLegal(s)

	open := rng.Intn(o.openMax + 1)
	for i := 0; i < open && len(moves) > 0; i++ {
		m := moves[rng.Intn(len(moves))]
		engine.MakeMove(s, m)
		moves = engine.GenerateLegal(s)
	}

	seen := map[[67]byte]int{}
	var out []nn.RawSample
	result := float32(0)
	discard := false
	for ply := 0; ply < o.maxPly; ply++ {
		if s.Halfmove >= 100 {
			result = 0
			break
		}
		moves := engine.GenerateLegal(s)
		if len(moves) == 0 {
			if engine.IsInCheck(s, s.Stm) {
				if s.Stm == engine.White {
					result = -1
				} else {
					result = 1
				}
			}
			break
		}
		key := posKey(s)
		if seen[key] >= 3 {
			result = 0
			break
		}
		seen[key]++
		global.add(key)

		out = append(out, nn.RawSample{Board: s.Board, Stm: int8(s.Stm)})
		m := engine.FindBestMoveWith(s, o.depth, 0, engine.NNConfig{})
		if len(moves) > 0 && rng.Float64() < o.novelty {
			m = pickDiverse(s, moves, global, rng)
		} else if ply < 24 && rng.Float64() < o.noise && len(moves) > 0 {
			m = moves[rng.Intn(len(moves))]
		}
		engine.MakeMove(s, m)
		if ply == o.maxPly-1 {
			discard = true
			break
		}
	}

	if discard {
		return nil, 0, true
	}
	if !o.mirror {
		for i := range out {
			out[i].Target = result
		}
		return out, result, false
	}
	twice := make([]nn.RawSample, 0, len(out)*2)
	for _, r := range out {
		r.Target = result
		twice = append(twice, r)
		twice = append(twice, nn.RawSample{Board: mirrorBoard(r.Board), Stm: -r.Stm, Target: result})
	}
	return twice, result, false
}

///
/// <summary>
///   stats accumulates game outcome counters across workers.
/// </summary>
type stats struct {
	whiteWins atomic.Int64
	blackWins atomic.Int64
	draws     atomic.Int64
	discarded atomic.Int64
	positions atomic.Int64
	games     atomic.Int64
}

///
/// <summary>
///   worker plays games until the shared counter reaches the target and
///   forwards the samples to the collector channel.
/// </summary>
/// <param name="o">Global options.</param>
/// <param name="st">Shared statistics.</param>
/// <param name="global">Shared position counter driving novelty.</param>
/// <param name="out">Sample channel.</param>
/// <param name="w">WaitGroup entry to signal.</param>
func worker(o options, st *stats, global *seenPos, out chan<- []nn.RawSample, w *sync.WaitGroup) {
	defer w.Done()
	rng := rand.New(rand.NewSource(o.seed + st.games.Load()))
	for {
		if o.pos > 0 && st.positions.Load() >= o.pos {
			return
		}
		n := st.games.Add(1)
		if n > int64(o.games) {
			return
		}
		samples, result, discard := playGame(rng, o, global)
		if discard {
			st.discarded.Add(1)
			continue
		}
		switch {
		case result > 0:
			st.whiteWins.Add(1)
		case result < 0:
			st.blackWins.Add(1)
		default:
			st.draws.Add(1)
		}
		st.positions.Add(int64(len(samples)))
		out <- samples
	}
}

///
/// <summary>
///   main runs generation and writes the dataset, printing outcome stats.
/// </summary>
func main() {
	o := parseOptions()
	if o.games <= 0 || o.cores <= 0 {
		log.Fatal("games and cores must be positive")
	}

	out := make(chan []nn.RawSample, o.cores*2)
	var wg sync.WaitGroup
	st := &stats{}
	global := newSeenPos()
	for i := 0; i < o.cores; i++ {
		wg.Add(1)
		go worker(o, st, global, out, &wg)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	f, err := os.Create(o.out)
	if err != nil {
		log.Fatal(err)
	}
	written := 0
	start := time.Now()
	ticker := time.NewTicker(5 * time.Second)
	quit := make(chan struct{})
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				n := st.games.Load()
				fmt.Printf("games %d/%d  pos %d  (%.0f/s)\n", n, o.games, st.positions.Load(),
					float64(st.positions.Load())/time.Since(start).Seconds())
			case <-quit:
				ticker.Stop()
				done <- struct{}{}
				return
			}
		}
	}()

	enc := nn.NewDatasetWriter(f)
	for batch := range out {
		if err := enc.Write(batch); err != nil {
			log.Fatal(err)
		}
		written += len(batch)
	}
	if err := enc.Close(); err != nil {
		log.Fatal(err)
	}
	quit <- struct{}{}
	<-done
	f.Close()

	fmt.Printf("done: %d games  %d positions  %.0f pos/s\n", st.games.Load(), written,
		float64(written)/time.Since(start).Seconds())
	fmt.Printf("whiteWins %d  blackWins %d  draws %d  discarded %d\n",
		st.whiteWins.Load(), st.blackWins.Load(), st.draws.Load(), st.discarded.Load())
}