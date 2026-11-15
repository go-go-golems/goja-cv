// 02-real-scan: a real film scan (Epson V600 DNG, raw0002.dng) through the zoom and blur patterns (GCV-007 step 4).
// Run: goja-cv bench --root WS --read /Users/manuel/Dropbox/Photos/RAW/Scans/2024/10/2024-10-05 --repeat 3 02-real-scan.js
// --- open (header only)
mem.img = cv.open("/Users/manuel/Dropbox/Photos/RAW/Scans/2024/10/2024-10-05/raw0002.dng", { encoding: "linear" })
return (await mem.img).frame
// --- whole-image stats (decodes)
return (await mem.img.stats()).p99
// --- speck view, first 64 px window
return (await mem.img.view("speck").crop([2000, 2000, 64, 64]).stats()).max
// --- 19 more windows of the speck view
const out = []
for (let i = 1; i < 20; i++) out.push((await mem.img.view("speck").crop([(i * 1237) % 3400, (i * 743) % 3400, 64, 64]).stats()).max)
return out.length
// --- gaussian 2, 64 px window
return (await mem.img.gaussian(2).crop([2000, 2000, 64, 64]).stats()).max
// --- gaussian 8, 64 px window
return (await mem.img.gaussian(8).crop([2000, 2000, 64, 64]).stats()).max
