// replay of pi annotator on tile d1a55e986363697c_2048_1024: 38 evals in its order (GCV-007; helpers via --prelude)
// --- eval 1
mem.tile("d1a55e986363697c_2048_1024", 2048, 1024)
const v = mem.views()
return { scene: v.scene.look("scene"), speck: v.speck.look("speck"), speckColor: v.speckColor.look("speckColor") }
// --- eval 2
const h = await mem.hot()
mem.hotLook(40)
return { n: h.length, top: h.slice(0, 40) }
// --- eval 3
mem.sheet(mem.hotRows.slice(0, 15), 40, "hot 0-14")
mem.sheet(mem.hotRows.slice(15, 30), 40, "hot 15-29")
return mem.hotRows.slice(0, 30).map(o => o.id)
// --- eval 4
mem.zoom(2497, 1501, 30)
mem.zoom(3023, 1519, 30)
return "two zooms"
// --- eval 5
const p1 = await mem.probe(3030, 1520)
const p2 = await mem.probeR(3030, 1520, 10)
const near = mem.hotRows.filter(o => Math.hypot(o.cx-3030, o.cy-1520) < 30)
return { p1, p2, near }
// --- eval 6
mem.zoom(2865, 1696, 30)
mem.zoom(2490, 1621, 30)
return "zooms 3-4"
// --- eval 7
mem.zoom(2362, 1426, 30)
mem.zoom(2412, 1970, 30)
return "zooms 5-6"
// --- eval 8
mem.zoom(2950, 1954, 30)
mem.zoom(2812, 2014, 30)
return "zooms 7-8"
// --- eval 9
mem.zoom(2656, 1911, 30)
mem.zoom(2822, 1766, 30)
return "zooms 9-10"
// --- eval 10
mem.sweep(0); mem.sweep(1); mem.sweep(2); mem.sweep(3)
return "sweeps 0-3"
// --- eval 11
const a = mem.sweep(0), b = mem.sweep(1)
return [a, b]
// --- eval 12
const a = await mem.sweep(0), b = await mem.sweep(1)
return [a, b]
// --- eval 13
const rows = mem.hotRows.filter(o => o.sp >= 0.15)
const a = await mem.sweep(2), b = await mem.sweep(3)
return { count: rows.length, ids: rows.map(o => o.id + ":" + o.cx.toFixed(0) + "," + o.cy.toFixed(0) + " sp" + o.sp.toFixed(2)), looks: [a, b] }
// --- eval 14
mem.zoom(2095, 1070, 45)
mem.zoom(2317, 1085, 40)
return "zooms 11-12"
// --- eval 15
mem.zoom(2454, 1029, 30)
mem.zoom(3004, 1029, 30)
return "zooms 13-14"
// --- eval 16
mem.zoom(2269, 1133, 30)
mem.zoom(2905, 1180, 30)
return "zooms 15-16"
// --- eval 17
mem.zoom(2981, 1177, 30)
mem.zoom(2997, 1256, 30)
return "zooms 17-18"
// --- eval 18
mem.zoom(2927, 1280, 30)
mem.zoom(2814, 1276, 30)
return "zooms 19-20"
// --- eval 19
mem.zoom(2340, 1370, 40)
mem.zoom(2741, 1352, 30)
return "zooms 21-22"
// --- eval 20
mem.zoom(2218, 1679, 30)
mem.zoom(2082, 1359, 30)
return "zooms 23-24"
// --- eval 21
mem.zoom(2068, 1960, 50)
return "zoom 25"
// --- eval 22
mem.zoom(2305, 1852, 40)
mem.zoom(2640, 1775, 40)
return "zooms 26-27"
// --- eval 23
mem.zoom(2497, 1502, 16)
mem.zoom(3062, 1173, 22)
return "zooms 28-29"
// --- eval 24
mem.zoom(2494, 1505, 45)
const rows = mem.hotRows.filter(o => o.sp >= 0.12 && o.sp < 0.15)
return [rows.length, rows.map(o => o.id + ":" + o.cx.toFixed(0) + "," + o.cy.toFixed(0) + " sp" + o.sp.toFixed(2) + (o.edge?" E":""))]
// --- eval 25
mem.zoom(2490, 1512, 60)
return "zoom 30"
// --- eval 26
mem.zoom(2497, 1476, 22)
mem.zoom(2450, 1533, 22)
return "zooms 31-32"
// --- eval 27
mem.zoom(2488, 1905, 40)
mem.zoom(2920, 1590, 40)
return "zooms 33-34"
// --- eval 28
const rows = mem.hotRows.filter(o => o.sp >= 0.12 && o.sp < 0.15)
mem.sheet(rows.slice(0, 15), 40, "mid 0-14")
mem.sheet(rows.slice(15, 30), 40, "mid 15-29")
mem.sheet(rows.slice(30, 42), 40, "mid 30-41")
return [rows.slice(0,15).map(o=>o.id), rows.slice(15,30).map(o=>o.id), rows.slice(30).map(o=>o.id)]
// --- eval 29
mem.zoom(2240, 1577, 20)
mem.zoom(3030, 1523, 30)
return "zooms 35-36"
// --- eval 30
const v = mem.views()
const strip = v.scene.crop([2940, 1024, 132, 1024])
return { ops: Object.keys(strip).slice(0, 40), help: cv.help("crop") }
// --- eval 31
const v = mem.views()
const c = v.scene.crop([2940, 1024, 132, 1024])
return { c: String(c), h: cv.help("levels") }
// --- eval 32
const v = mem.views()
const right = v.scene.crop([2920, 1024, 152, 1024]).levels({ auto: true, gamma: 0.8 })
const top = v.scene.crop([2048, 1024, 1024, 140]).levels({ auto: true, gamma: 0.8 })
return [right.look("right strip"), top.look("top strip")]
// --- eval 33
const v = mem.views()
const a = v.scene.crop([2960, 1024, 112, 512]).levels({ auto: true, gamma: 0.8 })
const b = v.scene.crop([2960, 1536, 112, 512]).levels({ auto: true, gamma: 0.8 })
return [a.look({ label: "right upper", zoom: 3 }), b.look({ label: "right lower", zoom: 3 })]
// --- eval 34
const a = await mem.probe(2470, 1630)
const b = await mem.probe(2232, 1770)
const c = await mem.probe(2470, 1975)
return [a, b, c]
// --- eval 35
const pts = [[2865,1696],[2490,1621],[2412,1970],[2368,1427],[2357,1426],[2812,2015],[2741,1353],[2364,1369],[2950,1955],[2324,1358],[2318,1382],[2814,1277],[2240,1577],[2656,1911],[2822,1767],[3033,1520],[2499,1502]]
const out = []
for (const p of pts) out.push(await mem.probe(p[0], p[1]))
return out
// --- eval 36
const v = mem.views()
const a = v.scene.crop([2048, 1996, 512, 52]).levels({ auto: true, gamma: 0.8 })
const b = v.scene.crop([2560, 1996, 512, 52]).levels({ auto: true, gamma: 0.8 })
return [a.look({ label: "bottom left", zoom: 2 }), b.look({ label: "bottom right", zoom: 2 })]
// --- eval 37
const v = mem.views()
const a = v.scene.crop([2048, 1150, 90, 450]).levels({ auto: true, gamma: 0.8 })
const b = v.scene.crop([2048, 1600, 90, 448]).levels({ auto: true, gamma: 0.8 })
return [a.look({ label: "left mid", zoom: 2 }), b.look({ label: "left low", zoom: 2 })]
// --- eval 38
mem.zoom(2637, 1475, 30)
mem.zoom(2492, 1516, 18)
return "zooms 37-38"
