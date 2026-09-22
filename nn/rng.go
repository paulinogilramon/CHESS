package nn

import "math/rand"

///
/// <summary>
///   newRand returns a deterministic source of normal-distributed values for
///   network initialisation.
/// </summary>
/// <param name="seed">Seed for the generator.</param>
/// <returns>A normal source backed by a seeded PRNG.</returns>
func newRand(seed int64) *rand.Rand {
	return rand.New(rand.NewSource(seed))
}