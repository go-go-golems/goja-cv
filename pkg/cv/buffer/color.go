package buffer

import "math"

var (
	srgbToLinear8  [256]float32
	srgbToLinear16 [65536]float32
	// perceptual <-> linear tables over [0, 1] with linear interpolation.
	toPercLUT [lutSize + 1]float32
	toLinLUT  [lutSize + 1]float32
)

const lutSize = 4096

func init() {
	for i := range srgbToLinear8 {
		srgbToLinear8[i] = float32(SRGBToLinear(float64(i) / 255))
	}
	for i := range srgbToLinear16 {
		srgbToLinear16[i] = float32(SRGBToLinear(float64(i) / 65535))
	}
	for i := 0; i <= lutSize; i++ {
		x := float64(i) / lutSize
		toPercLUT[i] = float32(LinearToSRGB(x))
		toLinLUT[i] = float32(SRGBToLinear(x))
	}
}

// SRGBToLinear is the sRGB decoding transfer function, extended to all reals:
// odd-symmetric below zero and continuing the power segment above one.
func SRGBToLinear(v float64) float64 {
	if v < 0 {
		return -SRGBToLinear(-v)
	}
	if v <= 0.04045 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

// LinearToSRGB is the sRGB encoding transfer function, extended like SRGBToLinear.
func LinearToSRGB(v float64) float64 {
	if v < 0 {
		return -LinearToSRGB(-v)
	}
	if v <= 0.0031308 {
		return v * 12.92
	}
	return 1.055*math.Pow(v, 1/2.4) - 0.055
}

// Linear8 decodes an 8-bit sRGB sample to linear light.
func Linear8(u uint8) float32 { return srgbToLinear8[u] }

// Linear16 decodes a 16-bit sRGB sample to linear light.
func Linear16(u uint16) float32 { return srgbToLinear16[u] }

// ToPerceptual converts a linear value to the perceptual (sRGB-encoded) scale
// used by perceptual ops. Values in [0, 1] use an interpolated table (error
// below 2e-5); others use the exact extended transfer function.
func ToPerceptual(v float32) float32 {
	if v >= 0 && v <= 1 {
		return lutInterp(&toPercLUT, v)
	}
	return float32(LinearToSRGB(float64(v)))
}

// ToLinear is the inverse of ToPerceptual.
func ToLinear(p float32) float32 {
	if p >= 0 && p <= 1 {
		return lutInterp(&toLinLUT, p)
	}
	return float32(SRGBToLinear(float64(p)))
}

func lutInterp(t *[lutSize + 1]float32, v float32) float32 {
	x := v * lutSize
	i := int(x)
	if i >= lutSize {
		return t[lutSize]
	}
	f := x - float32(i)
	return t[i] + (t[i+1]-t[i])*f
}

// EncodeSRGB8 converts a linear value to a clamped 8-bit sRGB sample.
func EncodeSRGB8(v float32) uint8 {
	p := LinearToSRGB(clamp01(float64(v)))
	return uint8(math.Round(p * 255))
}

// EncodeSRGB16 converts a linear value to a clamped 16-bit sRGB sample.
func EncodeSRGB16(v float32) uint16 {
	p := LinearToSRGB(clamp01(float64(v)))
	return uint16(math.Round(p * 65535))
}

func clamp01(v float64) float64 {
	if v < 0 || math.IsNaN(v) {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
