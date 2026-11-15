// replay of pi annotator on tile 61ba690de9a8be1e_512_15360: 61 evals in its order (GCV-007; helpers via --prelude)
// --- eval 1
mem.tile("61ba690de9a8be1e_512_15360", 512, 15360)
const v = mem.views()
return { scene: v.scene.look("scene"), speck: v.speck.look("speck"), speckColor: v.speckColor.look("speckColor") }
// --- eval 2
const h = await mem.hot()
mem.hotLook(40)
return { n: h.length, first: h.slice(0, 40) }
// --- eval 3
mem.sheet(mem.hotRows.slice(0, 15), 40, "hot 0-14")
mem.sheet(mem.hotRows.slice(15, 30), 40, "hot 15-29")
return mem.hotRows.slice(0, 30).map(o => o.id)
// --- eval 4
mem.zoom(1343.7, 15691)
mem.zoom(1284.1, 16240)
return "z1"
// --- eval 5
mem.zoom(1083, 15801)
mem.zoom(1041.4, 16262)
return "z2"
// --- eval 6
mem.zoom(836.94, 16245)
mem.zoom(798.75, 15978)
return "z3"
// --- eval 7
mem.zoom(1400.3, 15700)
mem.zoom(1434.6, 15885)
return "z4"
// --- eval 8
mem.zoom(992.75, 15605)
mem.zoom(931.13, 15864)
return "z5"
// --- eval 9
const rows = mem.hotRows
const lowchroma = rows.filter(o => o.sp >= 0.13 && (o.chroma === null || o.chroma === undefined || o.chroma < 0.6))
return { total: rows.length, ge15: rows.filter(o=>o.sp>=0.15).length, low: lowchroma }
// --- eval 10
const rows = mem.hotRows
const lc = rows.filter(o => o.sp >= 0.13 && (o.chroma === null || o.chroma === undefined || o.chroma < 0.6))
mem.sheet(lc.slice(0, 15), 40, "lc 0-14")
mem.sheet(lc.slice(15, 26), 40, "lc 15-25")
return lc.slice(18).map(o => o.id)
// --- eval 11
mem.zoom(812.74, 16368)
mem.zoom(758.99, 16307)
return "z6"
// --- eval 12
mem.sweep(0)
mem.sweep(1)
return "sweep 0,1"
// --- eval 13
mem.sweep(2)
mem.sweep(3)
return "sweep 2,3"
// --- eval 14
const s = mem.sweep(0)
return s
// --- eval 15
mem.sweep(1)
return "q1"
// --- eval 16
const a = mem.sweep(1)
return { a }
// --- eval 17
const a = await mem.sweep(1)
return { a }
// --- eval 18
mem.zoom(1332, 15456)
mem.zoom(1336, 15631)
return "z7"
// --- eval 19
const d = cv.docs.search("crop levels view look")
return d
// --- eval 20
return cv.docs.read("looks")
// --- eval 21
const sp = mem.img.view("speck")
const strip = sp.crop([1300, 15360, 90, 512]).levels({ auto: true })
return { s: strip.look({ label: "speck strip x1300-1390", zoom: 2, ruler: 20 }) }
// --- eval 22
mem.zoom(1250, 15900)
mem.zoom(1290, 16050)
return "z8"
// --- eval 23
const sc = mem.img.view("scene")
const s = await sc.stats()
const band = sc.crop([1180, 15400, 180, 700]).levels({ black: s.p10, white: s.p99 }).look({label:"scene band x1180-1360", zoom:2.5, ruler:20})
const bandc = mem.img.view("speckColor").crop([1180, 15400, 180, 700]).levels({auto:true}).look({label:"speckColor band", zoom:2.5, ruler:20})
return { s, band }
// --- eval 24
const sc = mem.img.view("scene")
const s = await sc.stats()
const band = sc.crop([1180, 15400, 180, 627]).levels({ black: s.p10, white: s.p99 }).look({label:"scene band x1180-1360", zoom:2.2, ruler:20})
const bandc = mem.img.view("speckColor").crop([1180, 15400, 180, 627]).levels({auto:true}).look({label:"speckColor band", zoom:2.2, ruler:20})
return { band }
// --- eval 25
const scCol = mem.img.view("speckColor")
const strip = scCol.crop([1290, 15360, 110, 1024]).levels({auto:true}).look({label:"speckColor strip x1290-1400 full height", zoom:1.5, ruler:20})
const stripS = mem.img.view("scene").crop([1290, 15360, 110, 1024]).levels({auto:true}).look({label:"scene strip x1290-1400 full height", zoom:1.5, ruler:20})
return { strip }
// --- eval 26
const scCol = mem.img.view("speckColor")
const c1 = scCol.crop([1300, 15680, 70, 190]).levels({auto:true}).look({label:"speckColor thread 1", zoom:8, ruler:10})
const sc2 = mem.img.view("speck")
const c2 = sc2.crop([1300, 15680, 70, 190]).levels({auto:true}).look({label:"speck thread 1", zoom:8, ruler:10})
return { c1 }
// --- eval 27
const sp = mem.img.view("speck")
const up = sp.crop([1290, 15360, 70, 200]).levels({auto:true}).look({label:"speck thread upper", zoom:7.5, ruler:10})
const dn = sp.crop([1290, 16180, 70, 200]).levels({auto:true}).look({label:"speck thread lower", zoom:7.5, ruler:10})
return { up }
// --- eval 28
const sp = mem.img.view("speck")
const a = sp.crop([1325, 15380, 45, 330]).levels({auto:true}).look({label:"speck thread A x1325-1370", zoom:4.5, ruler:10})
const b = mem.img.view("scene").crop([1325, 15380, 45, 330]).levels({auto:true}).look({label:"scene thread A x1325-1370", zoom:4.5, ruler:10})
return { a }
// --- eval 29
const a = await mem.sweep(2)
return { a }
// --- eval 30
return cv.docs.search("grayscale desaturate luminance channel rgb split")
// --- eval 31
return cv.docs.read("masks-and-regions")
// --- eval 32
const g = mem.img.toGray()
const s = await g.stats()
const strong = g.levels({ black: s.p30, white: s.p99.9 }).look({label:"gray strong stretch", maxSide: 1000})
return { s }
// --- eval 33
const g = mem.img.toGray()
const s = await g.stats()
const strong = g.levels({ black: s.p30, white: s["p99.9"] }).look({label:"gray strong stretch", maxSide: 900})
const mid = g.levels({ black: s.p60, white: s["p99"] }).look({label:"gray mid stretch", maxSide: 900})
return { s }
// --- eval 34
return cv.help("tophat")
// --- eval 35
const L = mem.lin.toGray()
mem.th = L.sub(L.erode(8).dilate(8))
const full = mem.th.levels({auto:true}).look({label:"tophat r8 linear gray", maxSide: 1000})
return { full }
// --- eval 36
const L = mem.lin.toGray()
mem.th4 = L.sub(L.erode(3).dilate(3))
const c = mem.th4.crop([1250, 15360, 160, 1024]).levels({auto:true}).look({label:"tophat r3 strip x1250-1410", zoom:1.4, ruler:20})
const d = mem.th4.crop([912, 15360, 160, 1024]).levels({auto:true}).look({label:"tophat r3 strip x912-1072", zoom:1.4, ruler:20})
return { c }
// --- eval 37
const sp = mem.img.view("speck")
const s1 = sp.crop([1320, 15440, 50, 480]).levels({auto:true}).look({label:"speck thread x1320-1370 y15440-15920", zoom:2.6, ruler:20})
const s2 = sp.crop([1330, 15440, 30, 160]).levels({auto:true}).look({label:"speck thread zoom x1330-1360 y15440-15600", zoom:8, ruler:10})
return { s1 }
// --- eval 38
const a = await mem.sweep(3)
return { a }
// --- eval 39
const rows = mem.hotRows
const hi = rows.filter(o => o.sp >= 0.15)
return hi.filter(o => o.chroma != null && o.chroma >= 0.6)
// --- eval 40
mem.zoom(1204.2, 15917)
mem.zoom(1156.1, 16367)
return "z9"
// --- eval 41
mem.zoom(679.31, 15604)
mem.zoom(602.38, 16050)
return "z10"
// --- eval 42
mem.zoom(1057.9, 15528)
mem.zoom(731.44, 16325)
return "z11"
// --- eval 43
const sc = mem.img.view("speckColor")
const a = sc.crop([600, 15560, 200, 200]).levels({auto:true}).look({label:"speckColor blue A", zoom:5, ruler:20})
const b = sc.crop([680, 15960, 200, 200]).levels({auto:true}).look({label:"speckColor blue B", zoom:5, ruler:20})
return { a }
// --- eval 44
const rows = mem.hotRows
const mid = rows.filter(o => o.sp >= 0.10 && o.sp < 0.15 && o.chroma != null && o.chroma < 0.55)
mem.sheet(mid.slice(0,15), 40, "mid 0-14")
mem.sheet(mid.slice(15,30), 40, "mid 15-29")
return { n: mid.length }
// --- eval 45
mem.zoom(1441, 15610)
mem.zoom(1198, 15718)
return "z12"
// --- eval 46
mem.mainRows = mem.hotRows
const h2 = await mem.hot(0.6)
const dark = h2.filter(o => o.cx > 1060 && o.sp >= 0.11)
return { n2: h2.length, darkN: dark.length, dark: dark.slice(0, 40) }
// --- eval 47
const d2 = mem.hotRows.filter(o => o.cx > 1060 && o.sp >= 0.075 && o.sp < 0.11)
return { n: d2.length, rows: d2.slice(0, 40) }
// --- eval 48
const d2 = mem.hotRows.filter(o => o.cx > 1060 && o.sp >= 0.088 && o.sp < 0.11)
mem.sheet(d2.slice(0, 15), 40, "dark 0-14")
mem.sheet(d2.slice(15, 30), 40, "dark 15-29")
return d2.length
// --- eval 49
const a = mem.th4.crop([512, 15360, 448, 512]).levels({auto:true}).look({label:"tophat r3 TL blue", maxSide: 1300})
const b = mem.th4.crop([512, 15872, 448, 512]).levels({auto:true}).look({label:"tophat r3 BL blue", maxSide: 1300})
return { a }
// --- eval 50
const out = []
for (let y = 15370; y <= 15720; y += 15) {
  const r = await mem.probe(1347, y)
  out.push({ y: r.cy, x: r.cx, sp: Math.round(r.sp*1000)/1000, snr: Math.round(r.snr*100)/100 })
}
return out
// --- eval 51
const out = []
for (let y = 15370; y <= 15720; y += 15) {
  const r = await mem.probe(1356, y)
  out.push({ y: Math.round(r.cy), x: Math.round(r.cx*10)/10, sp: Math.round(r.sp*1000)/1000, snr: Math.round(r.snr*100)/100, chroma: r.chroma })
}
return out
// --- eval 52
return cv.docs.read("measuring")
// --- eval 53
const sp = mem.img.view("speck")
const a = sp.crop([1330, 15420, 60, 130]).levels({auto:true}).look({label:"speck thread precise", zoom:10, ruler:10, grid:10})
return { a }
// --- eval 54
const pts = [[1354,15430],[1353,15440],[1352,15450],[1351,15460],[1350,15470],[1349,15480],[1348,15490]]
const out = []
for (const p of pts) { const r = await mem.probe(p[0], p[1]); out.push({ at: p, cx: Math.round(r.cx*10)/10, cy: Math.round(r.cy*10)/10, sp: Math.round(r.sp*1000)/1000, snr: Math.round(r.snr*100)/100, chroma: r.chroma }) }
return out
// --- eval 55
const e = mem.hotRows.filter(o => o.edge === true && o.sp >= 0.10)
return { n: e.length, rows: e.slice(0, 25) }
// --- eval 56
const ids = [1,3,6,11,13,15,25,29,71,5,8,41,10,20,52]
const out = []
for (const id of ids) {
  const row = mem.mainRows.find(o => o.id === id)
  const r = await mem.probe(row.cx, row.cy)
  out.push({ id, cx: Math.round(r.cx*10)/10, cy: Math.round(r.cy*10)/10, sp: Math.round(r.sp*1000)/1000, snr: Math.round(r.snr*100)/100, chroma: r.chroma, cover: Math.round(r.cover*100)/100 })
}
return out
// --- eval 57
mem.zoom(886.5, 15607)
mem.zoom(916.18, 15955)
return "z13"
// --- eval 58
mem.zoom(684.34, 15871)
mem.zoom(802.88, 15760)
return "z14"
// --- eval 59
const r = mem.hotRows.filter(o => (o.cx > 1470 || o.cx < 545) && o.sp >= 0.10)
return { n: r.length, rows: r.slice(0, 20) }
// --- eval 60
mem.zoom(532.83, 15890)
mem.zoom(516.31, 15789)
return "z15"
// --- eval 61
const r8 = mem.hotRows.filter(o => o.r >= 8 && o.sp >= 0.12)
return { n: r8.length, rows: r8.slice(0, 20) }
