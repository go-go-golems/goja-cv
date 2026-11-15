// scan-zoom: the speck view of the 24 MP bench scan, then 20 windows of 64 px, as an agent zooms around a scan.
// The scan: go run ./internal/testimage/gen -out DIR -bench-scan 6000x4000; then
//   goja-cv bench --root DIR examples/bench/scan-zoom.js
// --- open
mem.img = cv.open("bench-scan-6000x4000.npy", { encoding: "linear" })
return (await mem.img).frame
// --- speck view, first window
return await mem.img.view("speck").crop([3010, 2010, 64, 64]).stats()
// --- 19 more windows of the speck view
const out = []
for (let i = 1; i < 20; i++) {
  const x = (i * 1237) % 5900, y = (i * 743) % 3900
  out.push((await mem.img.view("speck").crop([x, y, 64, 64]).stats()).max)
}
return out.length
