// Package types provides core types used throughout the wasmvm package.
package types

// Fraction represents a rational number with a numerator and denominator.
type Fraction struct {
	Numerator   int64
	Denominator int64
}

// Mul multiplies the fraction by the given integer.
func (f *Fraction) Mul(m int64) Fraction {
	return Fraction{f.Numerator * m, f.Denominator}
}

// Floor returns the floor of the fraction as an integer.
func (f Fraction) Floor() int64 {
	return f.Numerator / f.Denominator
}

// UFraction represents an unsigned rational number with a numerator and denominator.
type UFraction struct {
	Numerator   uint64
	Denominator uint64
}

// Mul multiplies the unsigned fraction by the given unsigned integer.
func (f *UFraction) Mul(m uint64) UFraction {
	return UFraction{f.Numerator * m, f.Denominator}
}

// Floor returns the floor of the unsigned fraction as an unsigned integer.
func (f UFraction) Floor() uint64 {
	return f.Numerator / f.Denominator
}
