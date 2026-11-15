// big-blur: Gaussian blurs of the 24 MP bench scan, whole and as a 64 px window (GCV-006 §11.1-11.2).
// The scan: go run ./internal/testimage/gen -out DIR -bench-scan 6000x4000; then
//   goja-cv bench --root DIR examples/bench/big-blur.js
// A window of a blur costs the whole blur today: the evaluator computes every node over its frame.
// --- open and stats
mem.img = cv.open("bench-scan-6000x4000.npy", { encoding: "linear" })
return await mem.img.stats()
// --- crop of the source, 64 px
return await mem.img.crop([3000, 2000, 64, 64]).stats()
// --- gaussian 2, 64 px window
return await mem.img.gaussian(2).crop([3000, 2000, 64, 64]).stats()
// --- gaussian 8, 64 px window
return await mem.img.gaussian(8).crop([3000, 2000, 64, 64]).stats()
// --- gaussian 32, 64 px window
return await mem.img.gaussian(32).crop([3000, 2000, 64, 64]).stats()
// --- gaussian 32 again, another window (cached whole value)
return await mem.img.gaussian(32).crop([100, 100, 64, 64]).stats()
