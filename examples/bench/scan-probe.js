// scan-probe: peaks over the 24 MP bench scan, then measure the strongest 50, as a detection pass does.
// The scan: go run ./internal/testimage/gen -out DIR -bench-scan 6000x4000; then
//   goja-cv bench --root DIR examples/bench/scan-probe.js
// --- open
mem.img = cv.open("bench-scan-6000x4000.npy", { encoding: "linear" })
return (await mem.img).frame
// --- peaks, radius 2 4 8
mem.peaks = await mem.img.peaks({ radius: [2, 4, 8], snr: 3 })
return mem.peaks.length
// --- measure the strongest 50
return (await mem.img.measure(mem.peaks.slice(0, 50), { prefix: "probe" })).length
