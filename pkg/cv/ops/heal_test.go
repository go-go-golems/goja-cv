package ops

import (
	"math"
	"testing"

	"github.com/go-go-golems/goja-cv/internal/testimage"
	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/kernels"
)

// perceptualLuminance is sRGB_encode of the Rec. 709 luminance of linear b.
func perceptualLuminance(b *buffer.Buffer) []float32 {
	L := b.Luminance()
	out := make([]float32, len(L))
	for i, v := range L {
		out[i] = float32(buffer.LinearToSRGB(float64(v)))
	}
	return out
}

// speckDeviation is, for each planted speck, |mean perceptual luminance
// over the speck − mean over the true background there|.
func speckDeviation(t *testing.T, b *buffer.Buffer, p testimage.Planted) []float64 {
	t.Helper()
	L := perceptualLuminance(b)
	trueL := func(x, y float64) float64 {
		c := testimage.PlantedBackground(x, y)
		lin := buffer.LumR*buffer.SRGBToLinear(c[0]) + buffer.LumG*buffer.SRGBToLinear(c[1]) + buffer.LumB*buffer.SRGBToLinear(c[2])
		return buffer.LinearToSRGB(lin)
	}
	out := make([]float64, len(p.Specks))
	for k, s := range p.Specks {
		var d float64
		var n int
		for y := int(s.Y - s.RY); y <= int(s.Y+s.RY); y++ {
			for x := int(s.X - s.RX); x <= int(s.X+s.RX); x++ {
				dx, dy := (float64(x)+0.5-s.X)/s.RX, (float64(y)+0.5-s.Y)/s.RY
				if dx*dx+dy*dy > 1 {
					continue
				}
				d += float64(L[y*b.W+x]) - trueL(float64(x)+0.5, float64(y)+0.5)
				n++
			}
		}
		out[k] = math.Abs(d / float64(max(1, n)))
	}
	return out
}

func stat(xs []float64) (float64, float64) {
	mean, worst := 0.0, 0.0
	for _, x := range xs {
		mean += x
		worst = math.Max(worst, x)
	}
	return mean / float64(len(xs)), worst
}

func TestHealRemovesPlantedSpecks(t *testing.T) {
	e, img, p := plantedEngine(t)
	keep := call(t, e, "where", call(t, e, "detect.specks", img.ID).ID, map[string]any{"score": map[string]any{"gte": DustScore}})
	// the deviations are computed from linear buffers: heal is physical, so
	// as(healed, "linear") is heal's own result
	linear := func(n *graph.Node) *buffer.Buffer {
		return evalValue(t, e, call(t, e, graph.AsOp, n.ID, "linear"), 1).(*graph.ImageValue).Buf
	}
	src := linear(img)
	before, worstBefore := stat(speckDeviation(t, src, p))
	for _, method := range []string{"patch", "diffuse"} {
		healed := call(t, e, "heal", img.ID, graph.NodeArg{ID: keep.ID}, map[string]any{"radius": 1.5, "method": method})
		hb := linear(healed)
		after, worst := stat(speckDeviation(t, hb, p))
		t.Logf("%s: speck deviation from background mean %.4f (worst %.4f) → %.4f (worst %.4f); noise σ %.4f",
			method, before, worstBefore, after, worst, testimage.PlantedNoise)
		if after > 1.5*testimage.PlantedNoise || worst > 0.02 {
			t.Fatalf("%s: specks not healed: mean %.4f worst %.4f", method, after, worst)
		}
		// nothing changes away from the (grown) targets
		tm := evalValue(t, e, call(t, e, "toMask", keep.ID, map[string]any{"grow": 2.5}), 1).(*graph.ImageValue).Buf.Plane(0)
		for i := range hb.Pix {
			if tm[i%len(tm)] == 0 && hb.Pix[i] != src.Pix[i] {
				t.Fatalf("%s changed pixel %d outside the targets", method, i)
			}
		}
		// the healed scan is clean
		cd := evalValue(t, e, call(t, e, "check.dust", healed.ID), 1).(*graph.DataValue).Data.(map[string]any)
		if cd["count"].(float64) > 1 {
			t.Fatalf("%s: check.dust after heal: %v", method, cd)
		}
		// heal at preview scale is consistent (same regions, rasterized at 0.5)
		if v := evalValue(t, e, healed, 0.5).(*graph.ImageValue); v.Buf.W != 128 || v.Frame.W != 256 {
			t.Fatalf("%s at 0.5: %v", method, v.Buf)
		}
	}
	// patch keeps grain: the residual noise inside healed specks is near σ
	grain := func(method string) float64 {
		hb := linear(call(t, e, "heal", img.ID, graph.NodeArg{ID: keep.ID}, map[string]any{"radius": 1.5, "method": method}))
		L := perceptualLuminance(hb)
		var vals []float32
		for _, s := range p.Specks {
			for y := int(s.Y - s.RY); y <= int(s.Y+s.RY); y++ {
				for x := int(s.X - s.RX); x <= int(s.X+s.RX); x++ {
					// high-pass: pixel minus its 4-neighbour mean
					i := y*hb.W + x
					vals = append(vals, L[i]-(L[i-1]+L[i+1]+L[i-hb.W]+L[i+hb.W])/4)
				}
			}
		}
		_, s := kernels.MAD(vals)
		return s
	}
	gp, gd := grain("patch"), grain("diffuse")
	t.Logf("high-pass σ inside healed specks: patch %.4f, diffuse %.4f", gp, gd)
	if !(gp > 2*gd) {
		t.Fatalf("patch should keep grain: %.4f vs diffuse %.4f", gp, gd)
	}
}

func TestInpaintMask(t *testing.T) {
	e, img := newEngine(t)
	m := call(t, e, "mask.box", img.ID, []any{20, 15, 12, 10})
	out := evalValue(t, e, call(t, e, "inpaint", img.ID, graph.NodeArg{ID: m.ID}), 1).(*graph.ImageValue).Buf
	src := evalValue(t, e, img, 1).(*graph.ImageValue).Buf
	// the gradient image is harmonic (linear per channel in perceptual terms,
	// nearly so in linear light): the fill is close to the original
	var worst float64
	for c := 0; c < 3; c++ {
		for y := 15; y < 25; y++ {
			for x := 20; x < 32; x++ {
				i := c*64*48 + y*64 + x
				worst = math.Max(worst, math.Abs(float64(out.Pix[i]-src.Pix[i])))
			}
		}
	}
	if worst > 0.03 {
		t.Fatalf("inpaint error %g", worst)
	}
}
