package bow

import (
	"fmt"
	"math"
	"strings"

	"github.com/TuftsBCB/fragbag"
)

// Bow represents a bag-of-words vector of size N for a particular fragment
// library, where N corresponds to the number of fragments in the fragment
// library.
//
// Note that a Bow may be weighted. It is up to the fragment library to
// apply weights to a Bow.
type Bow struct {
	// Freqs is a map from fragment number to the number of occurrences of
	// that fragment in this "bag of words." This map always has size
	// equivalent to the size of the library.
	Freqs []float32
}

// NewBow returns a bag-of-words with all fragment frequencies set to 0.
func NewBow(size int) Bow {
	b := Bow{
		Freqs: make([]float32, size),
	}
	for i := 0; i < size; i++ {
		b.Freqs[i] = 0
	}
	return b
}

// Weighted transforms any Bow into a weighted Bow with the scheme in the given
// weighted fragment library. The Bow size must be equivalent to the size of
// the library given.
func (b Bow) Weighted(lib fragbag.WeightedLibrary) Bow {
	if b.Len() != lib.Size() {
		panic(fmt.Sprintf("Cannot weight Bow with a library of a different "+
			"size. Bow has size %d while library (%s) has size %d.",
			b.Len(), lib.Name(), lib.Size()))
	}

	weighted := NewBow(b.Len())
	for i := 0; i < weighted.Len(); i++ {
		weighted.Freqs[i] = lib.AddWeights(i, b.Freqs[i])
	}
	return weighted
}

// Len returns the size of the vector. This is always equivalent to the
// corresponding library's fragment size.
func (b Bow) Len() int {
	return len(b.Freqs)
}

// Equal tests whether two Bows are equal.
//
// Two Bows are equivalent when the frequencies of every fragment are equal.
func (b Bow) Equal(b2 Bow) bool {
	if b.Len() != b2.Len() {
		return false
	}
	for i, freq1 := range b.Freqs {
		if freq1 != b2.Freqs[i] {
			return false
		}
	}
	return true
}

// Add performs an add operation on each fragment frequency and returns
// a new Bow. Add will panic if the operands have different lengths.
func (b Bow) Add(b2 Bow) Bow {
	if b.Len() != b2.Len() {
		panic("Cannot add two Bows with differing lengths")
	}

	sum := NewBow(b.Len())
	for i := 0; i < sum.Len(); i++ {
		sum.Freqs[i] = b.Freqs[i] + b2.Freqs[i]
	}
	return sum
}

// Euclid returns the euclidean distance between b and b2.
func (b Bow) Euclid(b2 Bow) float64 {
	f1, f2 := b.Freqs, b2.Freqs
	squareSum := float32(0)
	libsize := b.Len()
	for i := 0; i < libsize; i++ {
		squareSum += (f2[i] - f1[i]) * (f2[i] - f1[i])
	}
	return math.Sqrt(float64(squareSum))
}

// Cosine returns the cosine distance between b and b2.
func (b Bow) Cosine(b2 Bow) float64 {
	// This function is a hot-spot, so we manually inline the Dot
	// and Magnitude computations.

	var dot, mag1, mag2 float32
	libs := len(b.Freqs)
	freqs1, freqs2 := b.Freqs, b2.Freqs

	var f1, f2 float32
	for i := 0; i < libs; i++ {
		f1, f2 = freqs1[i], freqs2[i]
		dot += f1 * f2
		mag1 += f1 * f1
		mag2 += f2 * f2
	}
	r := 1.0 - (float64(dot) / math.Sqrt(float64(mag1)*float64(mag2)))
	if math.IsNaN(r) {
		return 1.0
	}
	return r
}

// Dot returns the dot product of b and b2.
func (b Bow) Dot(b2 Bow) float64 {
	dot := float32(0)
	libsize := b.Len()
	f1, f2 := b.Freqs, b2.Freqs
	for i := 0; i < libsize; i++ {
		dot += f1[i] * f2[i]
	}
	return float64(dot)
}

// Magnitude returns the vector length of b.
func (b Bow) Magnitude() float64 {
	mag := float32(0)
	libsize := b.Len()
	fs := b.Freqs
	for i := 0; i < libsize; i++ {
		mag += fs[i] * fs[i]
	}
	return math.Sqrt(float64(mag))
}

// String returns a string representation of the Bow vector. Only fragments
// with non-zero frequency are emitted.
//
// The output looks like '{fragNum: frequency, fragNum: frequency, ...}'.
// i.e., '{1: 4, 3: 1}' where all fragment numbers except '1' and '3' have
// a frequency of zero.
func (b Bow) String() string {
	pieces := make([]string, 0, 10)
	for i := 0; i < b.Len(); i++ {
		freq := b.Freqs[i]
		if freq > 0 {
			pieces = append(pieces, fmt.Sprintf("%d: %f", i, freq))
		}
	}
	return fmt.Sprintf("{%s}", strings.Join(pieces, ", "))
}
