package dm

import (
	"math"
)

type Float32UnaryFunction interface {
	Compute(input float64) float64
}

type GeneralizedLogisticFunction struct {
	A, K float64 // Lower and upper asymptotes which the output will approach.
	M    float64 // Origin of input.
	B    float64 // Exponential growth rate, used in: e ^ (-B(t-M))
	Q    float64 // Scalar growth rate, and Y(M) anchor, used in denominator: 1 + Q e ^ (-B(t-M))
	V    float64 // Asymmetry Factor (skews the curve left or right, so that one asymptote or another is "longer")
}

func (p *GeneralizedLogisticFunction) Compute(input float64) float64 {
	denominator := 1 + p.Q*math.Exp(-p.B*(float64(input)-p.M))
	if p.V != 1 {
		denominator = math.Pow(denominator, 1/p.V)
	}
	return p.A + (p.K-p.A)/denominator
}

func MakeSymmetricLogisticFunction(
	inputLo, inputHi, outputLo, outputHi float64) *GeneralizedLogisticFunction {

	inputRange := inputHi - inputLo
	inputMid := (inputLo + inputHi) / 2

	outputRange := outputHi - outputLo
	//	outputMid := (outputLo + outputHi) / 2

	p := &GeneralizedLogisticFunction{
		A: outputLo,
		K: outputHi,
		M: inputMid,
		B: 10 * outputRange / inputRange,
		Q: 1,
		V: 1,
	}
	return p
}
