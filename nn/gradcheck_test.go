package nn

import (
	"math"
	"testing"
)

///
/// <summary>
///   lossFor computes the half mean-squared error of a single sample.
/// </summary>
/// <param name="n">Network to evaluate.</param>
/// <param name="feats">Sparse features.</param>
/// <param name="target">Supervisory target.</param>
/// <returns>0.5*(out-target)^2.</returns>
func lossFor(n *Net, feats []uint16, target float32) float64 {
	out := n.Predict(feats)
	d := float64(out) - float64(target)
	return 0.5 * d * d
}

///
/// <summary>
///   TestBackwardNumerical compares the analytic gradients against central
///   finite differences on a single sample for every trainable parameter.
/// </summary>
/// <param name="t">Test context.</param>
func TestBackwardNumerical(t *testing.T) {
	n := NewNet(6, 4, 2)
	feats := []uint16{0, 1, 14, 40, 41, 100, 200, 300, 500, 700}
	sample := Sample{Feats: feats, Target: 0.7}

	fw := forwardTrain(n, feats)
	g := newGradSet(n)
	backward(n, sample, fw, g)

	groups := []struct {
		name string
		sl   []float32
	}{
		{"W1", n.W1},
		{"B1", n.B1},
		{"W2", n.W2},
		{"B2", n.B2},
		{"W3", n.W3},
	}
	analytic := map[string]func(int) float64{
		"W1": func(i int) float64 { return g.gW1[i] },
		"B1": func(i int) float64 { return g.gB1[i] },
		"W2": func(i int) float64 { return g.gW2[i] },
		"B2": func(i int) float64 { return g.gB2[i] },
		"W3": func(i int) float64 { return g.gW3[i] },
	}

	const eps = 1e-3
	for _, grp := range groups {
		for i := range grp.sl {
			orig := grp.sl[i]
			grp.sl[i] = orig - float32(eps)
			lo := lossFor(n, feats, sample.Target)
			grp.sl[i] = orig + float32(eps)
			hi := lossFor(n, feats, sample.Target)
			grp.sl[i] = orig
			num := (hi - lo) / (2 * eps)
			an := analytic[grp.name](i)
			if math.Abs(num) > 1e-2 && math.Abs(an) > 1e-2 {
				if rel := math.Abs(num - an); rel/math.Abs(an) > 1e-2 {
					t.Fatalf("grad mismatch %s[%d]: analytic %.6f numeric %.6f", grp.name, i, an, num)
				}
			}
		}
	}
}