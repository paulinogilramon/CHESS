package nn

import (
	"math"
	"math/rand"
)

///
/// <summary>
///   Sample is one training example: the sparse input features of a position
///   and the supervisory target from the side to move's perspective.
/// </summary>
/// <param name="Feats">Sparse feature indices.</param>
/// <param name="Target">Target value: +1 win, 0 draw, -1 loss.</param>
type Sample struct {
	Feats  []uint16
	Target float32
}

///
/// <summary>
///   TrainConfig carries the hyperparameters for one training run.
/// </summary>
/// <param name="Epochs">Number of passes over the training set.</param>
/// <param name="Batch">Minibatch size for Adam updates.</param>
/// <param name="LR">Adam learning rate.</param>
/// <param name="Beta1">Adam first-moment decay.</param>
/// <param name="Beta2">Adam second-moment decay.</param>
/// <param name="Eps">Adam numerical stabiliser.</param>
/// <param name="Seed">Seed for the shuffle generator.</param>
type TrainConfig struct {
	Epochs int
	Batch  int
	LR     float32
	Beta1  float32
	Beta2  float32
	Eps    float32
	Seed   int64
}

///
/// <summary>
///   DefaultConfig returns sensible hyperparameters for this network size.
/// </summary>
/// <returns>A zeroed config filled with Adam defaults.</returns>
func DefaultConfig() TrainConfig {
	return TrainConfig{
		Epochs: 6,
		Batch:  512,
		LR:     1e-3,
		Beta1:  0.9,
		Beta2:  0.999,
		Eps:    1e-8,
		Seed:   42,
	}
}

///
/// <summary>
///   fwd caches the intermediate activations of one forward pass so the
///   backward pass can reuse them without recomputation.
/// </summary>
type fwd struct {
	h1pre []float32
	h1    []float32
	h2pre []float32
	h2    []float32
	out   float32
}

///
/// <summary>
///   forwardTrain computes a forward pass for training, retaining the
///   pre-activation values needed by backpropagation.
/// </summary>
/// <param name="n">Network being trained.</param>
/// <param name="feats">Sparse input features.</param>
/// <returns>Cached activations.</returns>
func forwardTrain(n *Net, feats []uint16) *fwd {
	f := &fwd{h1pre: make([]float32, n.H1), h1: make([]float32, n.H1), h2pre: make([]float32, n.H2), h2: make([]float32, n.H2)}
	for _, ftr := range feats {
		base := int(ftr) * n.H1
		for i := 0; i < n.H1; i++ {
			f.h1pre[i] += n.W1[base+i]
		}
	}
	for i := 0; i < n.H1; i++ {
		f.h1pre[i] += n.B1[i]
		f.h1[i] = relu(f.h1pre[i])
	}
	for i := 0; i < n.H1; i++ {
		if f.h1[i] == 0 {
			continue
		}
		base := i * n.H2
		for j := 0; j < n.H2; j++ {
			f.h2pre[j] += f.h1[i] * n.W2[base+j]
		}
	}
	for j := 0; j < n.H2; j++ {
		f.h2pre[j] += n.B2[j]
		f.h2[j] = relu(f.h2pre[j])
	}
	z := n.B3
	for j := 0; j < n.H2; j++ {
		z += f.h2[j] * n.W3[j]
	}
	f.out = float32(math.Tanh(float64(z)))
	return f
}

///
/// <summary>
///   relu applies the rectified linear activation.
/// </summary>
/// <param name="x">Input value.</param>
/// <returns>max(0, x).</returns>
func relu(x float32) float32 {
	if x > 0 {
		return x
	}
	return 0
}

///
/// <summary>
///   gradSet accumulates per-parameter gradients for a minibatch in float64
///   to preserve accuracy during add-up.
/// </summary>
type gradSet struct {
	gW1 []float64
	gB1 []float64
	gW2 []float64
	gB2 []float64
	gW3 []float64
	gB3 float64
}

///
/// <summary>
///   newGradSet allocates a zeroed gradient set matching the parameter shapes
///   of a network.
/// </summary>
/// <param name="n">Network whose parameters the gradients must fit.</param>
/// <returns>A zeroed gradient set.</returns>
func newGradSet(n *Net) *gradSet {
	return &gradSet{
		gW1: make([]float64, len(n.W1)),
		gB1: make([]float64, len(n.B1)),
		gW2: make([]float64, len(n.W2)),
		gB2: make([]float64, len(n.B2)),
		gW3: make([]float64, len(n.W3)),
	}
}

///
/// <summary>
///   zero resets all accumulated gradients.
/// </summary>
func (g *gradSet) zero() {
	for i := range g.gW1 {
		g.gW1[i] = 0
	}
	for i := range g.gB1 {
		g.gB1[i] = 0
	}
	for i := range g.gW2 {
		g.gW2[i] = 0
	}
	for i := range g.gB2 {
		g.gB2[i] = 0
	}
	for i := range g.gW3 {
		g.gW3[i] = 0
	}
	g.gB3 = 0
}

///
/// <summary>
///   backward accumulates the gradient of the mean-squared-error loss between
///   the network output and the sample target into g.
/// </summary>
/// <param name="n">Network whose activations were cached.</param>
/// <param name="s">Sample with the supervisory target.</param>
/// <param name="f">Cached forward activations.</param>
/// <param name="g">Gradient accumulator to update.</param>
func backward(n *Net, s Sample, f *fwd, g *gradSet) {
	dOut := float64(f.out) - float64(s.Target)
	dZ := dOut * (1 - float64(f.out)*float64(f.out))

	for j := 0; j < n.H2; j++ {
		g.gW3[j] += dZ * float64(f.h2[j])
	}
	g.gB3 += dZ

	for j := 0; j < n.H2; j++ {
		if f.h2[j] <= 0 {
			continue
		}
		e2 := dZ * float64(n.W3[j])
		g.gB2[j] += e2
		for i := 0; i < n.H1; i++ {
			g.gW2[i*n.H2+j] += e2 * float64(f.h1[i])
		}
	}

	e1 := make([]float64, n.H1)
	for i := 0; i < n.H1; i++ {
		if f.h1[i] <= 0 {
			continue
		}
		for j := 0; j < n.H2; j++ {
			e1[i] += (dZ * float64(n.W3[j])) * float64(n.W2[i*n.H2+j])
		}
	}
	for i := 0; i < n.H1; i++ {
		if e1[i] == 0 {
			continue
		}
		g.gB1[i] += e1[i]
		for _, ftr := range s.Feats {
			g.gW1[int(ftr)*n.H1+i] += e1[i]
		}
	}
}

///
/// <summary>
///   adamState tracks the first and second moments of Adam for every
///   parameter of a network, grouped by the parameter group order.
/// </summary>
type adamState struct {
	m []float64
	v []float64
	t int
}

///
/// <summary>
///   newAdam allocates Adam moment buffers sized for a network.
/// </summary>
/// <param name="n">Network to optimise.</param>
/// <returns>Zerced Adam state.</returns>
func newAdam(n *Net) *adamState {
	total := 1
	for _, g := range n.Params() {
		total += len(g)
	}
	return &adamState{m: make([]float64, total), v: make([]float64, total)}
}

///
/// <summary>
///   applyAdam performs one Adam update step on a single parameter group using
///   the accumulated batch gradient and the shared moment state.
/// </summary>
/// <param name="w">Parameter group base slice.</param>
/// <param name="base">Flat offset of the group in the moment buffers.</param>
/// <param name="a">Shared Adam state.</param>
/// <param name="grad">Accumulated batch gradients for this group.</param>
/// <param name="batch">Number of samples that contributed to the gradient.</param>
/// <param name="cfg">Hyperparameters.</param>
func applyAdam(w []float32, base int, a *adamState, grad []float64, batch int, cfg TrainConfig) {
	b1c, b2c := 1-math.Pow(float64(cfg.Beta1), float64(a.t)), 1-math.Pow(float64(cfg.Beta2), float64(a.t))
	for i := range w {
		g := grad[i] / float64(batch)
		a.m[base+i] = float64(cfg.Beta1)*a.m[base+i] + (1-float64(cfg.Beta1))*g
		a.v[base+i] = float64(cfg.Beta2)*a.v[base+i] + (1-float64(cfg.Beta2))*g*g
		mHat := a.m[base+i] / b1c
		vHat := a.v[base+i] / b2c
		w[i] -= float32(float64(cfg.LR) * mHat / (math.Sqrt(vHat) + float64(cfg.Eps)))
	}
}

///
/// <summary>
///   Train runs Adam minibatch SGD over the samples, reporting the mean
///   training and validation loss per epoch.
/// </summary>
/// <param name="n">Network to train in place.</param>
/// <param name="samples">Training samples (shuffled in place across epochs).</param>
/// <param name="val">Hold-out validation samples.</param>
/// <param name="cfg">Hyperparameters.</param>
/// <param name="progress">Optional logger invoked per minibatch with epoch, step and batch loss.</param>
/// <returns>The per-epoch training loss.</returns>
func Train(n *Net, samples []Sample, val []Sample, cfg TrainConfig, progress func(epoch, step int, loss float32)) []float32 {
	if cfg.Batch <= 0 {
		cfg.Batch = 512
	}
	rng := newRand(cfg.Seed)
	g := newGradSet(n)
	a := newAdam(n)
	epochLoss := make([]float32, cfg.Epochs)
	for epoch := 0; epoch < cfg.Epochs; epoch++ {
		shuffle(samples, rng)
		total := float64(0)
		count := 0
		for start := 0; start < len(samples); start += cfg.Batch {
			end := start + cfg.Batch
			if end > len(samples) {
				end = len(samples)
			}
			g.zero()
			batchLoss := float64(0)
			for k := start; k < end; k++ {
				s := samples[k]
				f := forwardTrain(n, s.Feats)
				diff := f.out - s.Target
				batchLoss += float64(diff * diff)
				backward(n, s, f, g)
			}
			a.t++
			applyAdam(n.W1, 0, a, g.gW1, end-start, cfg)
			applyAdam(n.B1, len(n.W1), a, g.gB1, end-start, cfg)
			applyAdam(n.W2, len(n.W1)+len(n.B1), a, g.gW2, end-start, cfg)
			applyAdam(n.B2, len(n.W1)+len(n.B1)+len(n.W2), a, g.gB2, end-start, cfg)
			applyAdam(n.W3, len(n.W1)+len(n.B1)+len(n.W2)+len(n.B2), a, g.gW3, end-start, cfg)

			total += batchLoss
			count += end - start
			if progress != nil {
				progress(epoch, start/cfg.Batch, float32(batchLoss/float64(end-start)))
			}
		}
		epochLoss[epoch] = float32(total / float64(count))
	}
	return epochLoss
}

///
/// <summary>
///   ValLoss computes the mean squared error on a held-out sample set.
/// </summary>
/// <param name="n">Network to evaluate.</param>
/// <param name="samples">Validation samples.</param>
/// <returns>Mean squared error.</returns>
func ValLoss(n *Net, samples []Sample) float32 {
	if len(samples) == 0 {
		return 0
	}
	total := float64(0)
	for _, s := range samples {
		v := n.Predict(s.Feats)
		diff := float64(v - s.Target)
		total += diff * diff
	}
	return float32(total / float64(len(samples)))
}

///
/// <summary>
///   shuffle permutes a sample slice in place using the provided generator.
/// </summary>
/// <param name="samples">Samples to permute.</param>
/// <param name="rng">Random source.</param>
func shuffle(samples []Sample, rng *rand.Rand) {
	for i := len(samples) - 1; i > 0; i-- {
		j := rng.Intn(i + 1)
		samples[i], samples[j] = samples[j], samples[i]
	}
}