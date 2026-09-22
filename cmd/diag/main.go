///
/// Command diag prints prediction and target statistics on a dataset so the
/// learned model can be sanity-checked (output scale, sign agreement).
package main

import (
	"flag"
	"fmt"
	"log"

	"chess/nn"
)

///
/// <summary>
///   main loads the weights and dataset and reports stats.
/// </summary>
func main() {
	var data, weights string
	var limit int
	flag.StringVar(&data, "data", "dataset.bin", "dataset path")
	flag.StringVar(&weights, "weights", "weights.bin", "weights path")
	flag.IntVar(&limit, "limit", 20000, "samples to inspect")
	flag.Parse()

	net, err := nn.Load(weights)
	if err != nil {
		log.Fatal(err)
	}
	samples, err := nn.LoadDataset(data)
	if err != nil {
		log.Fatal(err)
	}
	if limit > len(samples) {
		limit = len(samples)
	}

	var avgAbsOut, avgAbsT, agree, step10Mse, total int
	for i := 0; i < limit; i++ {
		s := samples[i]
		v := net.Predict(s.Feats)
		avgAbsOut += int(v * 100)
		avgAbsT += int(s.Target * 100)
		if (v > 0) == (s.Target > 0) && s.Target != 0 {
			agree++
		} else if s.Target == 0 {
			agree++
		}
		if i%10 == 0 {
			d := v - s.Target
			step10Mse += int(d * d * 100)
			total++
		}
	}
	f := float64(limit)
	fmt.Printf("mean|out|=%.2f mean|target|=%.2f signAgree=%.1f%%\n",
		float64(avgAbsOut)/f/100, float64(avgAbsT)/f/100, 100*float64(agree)/f)
	fmt.Printf("every-10th MSE=%.2f total=%d\n", float64(step10Mse)/float64(total)/100, total)
}