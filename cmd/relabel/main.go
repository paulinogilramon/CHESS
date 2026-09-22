///
/// Command relabel rewrites a self-play dataset, replacing each position's
/// game-outcome target with a search-derived value (centipawns, normalised)
/// so the network learns to imitate a stronger oracle instead of noisy final
/// results.
package main

import (
	"flag"
	"fmt"
	"log"
	"math"
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
	in     string
	out    string
	depth  int
	scale  float64
	step   int
	cores  int
	maxAbs int
}

///
/// <summary>
///   parseOptions reads the relabel command-line flags.
/// </summary>
/// <returns>Parsed options from os.Args, or exits on a bad flag.</returns>
func parseOptions() options {
	var o options
	flag.StringVar(&o.in, "in", "dataset.bin", "input dataset path")
	flag.StringVar(&o.out, "out", "relabel.bin", "output dataset path")
	flag.IntVar(&o.depth, "depth", 4, "search depth used to label positions")
	flag.Float64Var(&o.scale, "scale", 600, "centipawns mapped to a network unit")
	flag.IntVar(&o.step, "step", 2, "label one in every N raw samples")
	flag.IntVar(&o.cores, "cores", 4, "parallel worker goroutines")
	flag.IntVar(&o.maxAbs, "maxabs", 1200, "maximum absolute centipawn label")
	flag.Parse()
	return o
}

///
/// <summary>
///   toState reconstructs a minimal position from a raw sample; castling and
///   en-passant rights are dropped, which is acceptable for value labels.
/// </summary>
/// <param name="r">Raw sample.</param>
/// <returns>A playable engine state.</returns>
func toState(r nn.RawSample) *engine.State {
	return &engine.State{Board: r.Board, Stm: engine.Color(r.Stm), Castling: 0, Ep: -1, Halfmove: 0, Fullmove: 1}
}

///
/// <summary>
///   worker labels raw samples until the counter reaches the target count,
///   sending results as batches to amortise I/O.
/// </summary>
/// <param name="raw">All raw samples.</param>
/// <param name="o">Global options.</param>
/// <param name="idx">Shared index counter.</param>
/// <param name="out">Labeled sample channel.</param>
/// <param name="w">WaitGroup entry to signal.</param>
func worker(raw []nn.RawSample, o options, idx *atomic.Int64, out chan<- []nn.RawSample, w *sync.WaitGroup) {
	defer w.Done()
	batch := make([]nn.RawSample, 0, 1024)
	flush := func() {
		if len(batch) > 0 {
			out <- batch
			batch = make([]nn.RawSample, 0, 1024)
		}
	}
	defer flush()
	for {
		n := idx.Add(1)
		i := int((n - 1) * int64(o.step))
		if i >= len(raw) {
			return
		}
		s := toState(raw[i])
		v := engine.SearchValue(s, o.depth)
		if math.Abs(float64(v)) > float64(o.maxAbs) {
			if v > 0 {
				v = o.maxAbs
			} else {
				v = -o.maxAbs
			}
		}
		white := float32(v)
		if s.Stm == engine.Black {
			white = -white
		}
		r := raw[i]
		r.Target = white / float32(o.scale)
		batch = append(batch, r)
		if len(batch) >= 1024 {
			flush()
		}
	}
}

///
/// <summary>
///   main runs relabeling in parallel and writes the new dataset.
/// </summary>
func main() {
	o := parseOptions()
	raw, err := nn.LoadRawDataset(o.in)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("loaded %d raw samples", len(raw))

	total := int64(0)
	for i := 0; i < len(raw); i += o.step {
		total++
	}
	log.Printf("labelling %d positions (depth %d)", total, o.depth)

	out := make(chan []nn.RawSample, o.cores*64)
	idx := new(atomic.Int64)
	var wg sync.WaitGroup
	for i := 0; i < o.cores; i++ {
		wg.Add(1)
		go worker(raw, o, idx, out, &wg)
	}
	go func() {
		wg.Wait()
		close(out)
	}()

	f, err := os.Create(o.out)
	if err != nil {
		log.Fatal(err)
	}
	enc := nn.NewDatasetWriter(f)
	written := 0
	start := time.Now()
	nextLog := 5000
	for batch := range out {
		buf := make([]nn.RawSample, 0, len(batch))
		buf = append(buf, batch...)
		if err := enc.Write(buf); err != nil {
			log.Fatal(err)
		}
		written += len(buf)
		if written >= nextLog {
			fmt.Printf("wrote %d labelled positions in %.0fs\n", written, time.Since(start).Seconds())
			nextLog += 5000
		}
	}
	if err := enc.Close(); err != nil {
		log.Fatal(err)
	}
	f.Close()
	fmt.Printf("done: %d labelled positions in %.0fs\n", written, time.Since(start).Seconds())
}