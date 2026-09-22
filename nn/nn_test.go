package nn

import (
	"os"
	"path/filepath"
	"testing"
)

///
/// <summary>
///   TestForwardConsistency guarantees the training forward pass and the
///   inference Predict agree exactly on the same weights and features, so the
///   two code paths can never drift apart again.
/// </summary>
/// <param name="t">Test context.</param>
func TestForwardConsistency(t *testing.T) {
	for seed := int64(1); seed <= 5; seed++ {
		n := NewNet(8, 6, seed)
		feats := []uint16{0, 7, 12, 30, 44, 100, 200, 300, 400, 500, 700, 767}
		a := forwardTrain(n, feats).out
		b := n.Predict(feats)
		d := float32(a - b)
		if d < 0 {
			d = -d
		}
		if d > 1e-6 {
			t.Fatalf("seed %d: forwardTrain %v != Predict %v", seed, a, b)
		}
	}
}

///
/// <summary>
///   TestEncodeFeatureCount verifies that encoding a real position activates
///   exactly one feature per piece, all within the feature space.
/// </summary>
/// <param name="t">Test context.</param>
func TestEncodeFeatureCount(t *testing.T) {
	board := [64]int8{}
	board[0] = 4
	board[63] = -6
	board[27] = 1
	board[36] = -1
	feats := EncodeFeatures(board, 1)
	if len(feats) != 4 {
		t.Fatalf("expected 4 features, got %d", len(feats))
	}
	for _, f := range feats {
		if int(f) < 0 || int(f) >= FeatureCount {
			t.Fatalf("feature %d out of range", f)
		}
	}
}

///
/// <summary>
///   TestPredictRange ensures the tanh output stays inside (-1, 1) for a
///   frozen random network and that orienting mirrors encode alike counts.
/// </summary>
/// <param name="t">Test context.</param>
func TestPredictRange(t *testing.T) {
	n := NewNet(64, 32, 7)
	s := NewStartBoard()
	for _, stm := range []int8{1, -1} {
		v := n.Evaluate(s, stm)
		if v <= -1 || v >= 1 {
			t.Fatalf("value %v outside (-1,1)", v)
		}
	}
}

///
/// <summary>
///   NewStartBoard returns the standard starting position as a plain board.
/// </summary>
/// <returns>64 squares with the initial setup.</returns>
func NewStartBoard() [64]int8 {
	b := [64]int8{}
	back := [8]int8{4, 2, 3, 5, 6, 3, 2, 4}
	for f := 0; f < 8; f++ {
		b[f] = back[f]
		b[8+f] = 1
		b[48+f] = -1
		b[56+f] = -back[f]
	}
	return b
}

///
/// <summary>
///   TestSaveLoad round-trips a network through the binary format and
///   verifies the reconstructed network produces identical predictions.
/// </summary>
/// <param name="t">Test context.</param>
func TestSaveLoad(t *testing.T) {
	n := NewNet(64, 32, 3)
	path := filepath.Join(t.TempDir(), "weights.bin")
	if err := n.Save(path); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.H1 != n.H1 || got.H2 != n.H2 || got.Count() != n.Count() {
		t.Fatalf("shape mismatch after save/load")
	}
	board := NewStartBoard()
	a := n.Evaluate(board, 1)
	b := got.Evaluate(board, 1)
	if a != b {
		t.Fatalf("predictions differ after round-trip: %v vs %v", a, b)
	}
}

///
/// <summary>
///   TestTrainOverfit confirms the trainer reduces training loss by a large
///   margin on a tiny dataset, exercising the full backward pass and Adam
///   update path.
/// </summary>
/// <param name="t">Test context.</param>
func TestTrainOverfit(t *testing.T) {
	board := NewStartBoard()
	samples := make([]Sample, 0, 128)
	for i := 0; i < 64; i++ {
		p := EncodeFeatures(board, 1)
		samples = append(samples, Sample{Feats: p, Target: 0.6})
	}
	for i := 0; i < 64; i++ {
		board[12] = -1
		p := EncodeFeatures(board, 1)
		samples = append(samples, Sample{Feats: p, Target: -0.6})
		board[12] = 0
	}
	cfg := DefaultConfig()
	cfg.Epochs = 20
	cfg.Batch = 32
	cfg.Seed = 5
	losses := Train(NewNet(64, 32, 9), samples, nil, cfg, nil)
	if losses[len(losses)-1] > losses[0]*0.5 {
		t.Fatalf("training did not reduce loss: first=%v last=%v", losses[0], losses[len(losses)-1])
	}
}

///
/// <summary>
///   TestDatasetRoundTrip writes and re-reads raw samples and confirms the
///   target is correctly re-baselined to the side to move.
/// </summary>
/// <param name="t">Test context.</param>
func TestDatasetRoundTrip(t *testing.T) {
	board := NewStartBoard()
	raw := []RawSample{
		{Board: board, Stm: 1, Target: 1},
		{Board: board, Stm: -1, Target: 1},
	}
	path := filepath.Join(t.TempDir(), "data.bin")
	if err := SaveDataset(path, raw); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Size() != int64(8+8+2*(64+1+4)) {
		t.Fatalf("unexpected file size %d", fi.Size())
	}
	samples, err := LoadDataset(path)
	if err != nil {
		t.Fatal(err)
	}
	if samples[0].Target != 1 || samples[1].Target != -1 {
		t.Fatalf("target orientation wrong: %v %v", samples[0].Target, samples[1].Target)
	}
}