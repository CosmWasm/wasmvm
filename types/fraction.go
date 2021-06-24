package types

type Fraction struct {
	Numerator   int
	Denominator int
}

func (f *Fraction) Mul(m int) Fraction {
	return Fraction{f.Numerator * m, f.Denominator}
}

func (f Fraction) Int() int {
	return f.Numerator / f.Denominator
}
