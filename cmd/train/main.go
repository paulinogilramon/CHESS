///
/// Command train loads a self-play dataset, trains the small value network
/// with Adam minibatch SGD, and writes the learned weights to a binary file
/// consumed by the engine.
package main

import (
	"flag"
	"fmt"
	"log"

	"chess/nn"
)

///
/// <summary>
///   Options gathered from the command line.
/// </summary>
type options struct {
	data    string
	out     string
	epochs  int
	batch   int
	lr      float64
	h1      int
	h2      int
	valFrac float64
	seed    int64
}

///
/// <summary>
///   parseOptions reads the train command-line flags.
/// </summary>
/// <returns>Parsed options from os.Args, or exits on a bad flag.</returns>
func parseOptions() options {
	var o options
	flag.StringVar(&o.data, "data", "dataset.bin", "input dataset path")
	flag.StringVar(&o.out, "out", "weights.bin", "output weights path")
	flag.IntVar(&o.epochs, "epochs", 5, "training epochs")
	flag.IntVar(&o.batch, "batch", 512, "minibatch size")
	flag.Float64Var(&o.lr, "lr", 1e-3, "Adam learning rate")
	flag.IntVar(&o.h1, "h1", 64, "first hidden layer width")
	flag.IntVar(&o.h2, "h2", 32, "second hidden layer width")
	flag.Float64Var(&o.valFrac, "val", 0.05, "fraction of samples held out for validation")
	flag.Int64Var(&o.seed, "seed", 42, "shuffle seed")
	flag.Parse()
	return o
}

///
/// <summary>
///   splitValidation separates a regular sample slice into a training and a
///   held-out validation set, taking every k-th sample for validation.
/// </summary>
/// <param name="samples">All samples.</param>
/// <param name="frac">Validation fraction in (0,1).</param>
/// <returns>Training samples and validation samples.</returns>
func splitValidation(samples []nn.Sample, frac float64) ([]nn.Sample, []nn.Sample) {
	step := int(1 / frac)
	if step < 2 {
		step = 9
	}
	train := make([]nn.Sample, 0, len(samples))
	val := make([]nn.Sample, 0, len(samples)/step)
	for i, s := range samples {
		if i%step == 0 {
			val = append(val, s)
		} else {
			train = append(train, s)
		}
	}
	return train, val
}

///
/// <summary>
///   main loads the dataset, trains the network, and writes the weights.
/// </summary>
func main() {
	o := parseOptions()
	log.Printf("loading %s", o.data)
	samples, err := nn.LoadDataset(o.data)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("loaded %d samples", len(samples))

	train, val := splitValidation(samples, o.valFrac)
	log.Printf("train %d  val %d", len(train), len(val))

	net := nn.NewNet(o.h1, o.h2, o.seed)
	log.Printf("network %d->%d->%d->1  params %d", nn.FeatureCount, o.h1, o.h2, net.Count())

	cfg := nn.DefaultConfig()
	cfg.Epochs = o.epochs
	cfg.Batch = o.batch
	cfg.LR = float32(o.lr)
	cfg.Seed = o.seed

	step := 0
	losses := nn.Train(net, train, val, cfg, func(epoch, s int, loss float32) {
		step++
		if step%10 == 0 {
			fmt.Printf("epoch %d  step %d  loss %.4f\n", epoch, s, loss)
		}
	})
	for i, l := range losses {
		fmt.Printf("epoch %d  trainMSE %.4f  valMSE %.4f\n", i, l, nn.ValLoss(net, val))
	}
	vl := nn.ValLoss(net, val)
	fmt.Printf("val MSE %.4f\n", vl)

	if err := net.Save(o.out); err != nil {
		log.Fatal(err)
	}
	log.Printf("saved %s", o.out)
}