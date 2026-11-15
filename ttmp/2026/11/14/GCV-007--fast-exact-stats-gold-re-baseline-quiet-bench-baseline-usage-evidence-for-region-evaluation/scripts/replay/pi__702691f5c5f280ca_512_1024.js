// replay of pi annotator on tile 702691f5c5f280ca_512_1024: 18 evals in its order (GCV-007; helpers via --prelude)
// --- eval 1
const t = mem.tile("702691f5c5f280ca_512_1024", 512, 1024)
const v = mem.views()
return { ctx: Object.keys(mem), scene: v.scene.look("scene"), speck: v.speck.look("speck"), speckColor: v.speckColor.look("speckColor") }
// --- eval 2
const h = await mem.hot()
mem.hotLook(40)
return { n: h.length, rows: h.slice(0, 45) }
// --- eval 3
mem.sheet(mem.hotRows.slice(0, 15), 40, "hot 0-14")
mem.sheet(mem.hotRows.slice(15, 30), 40, "hot 15-29")
return { a: mem.hotRows.slice(0,15).map(o=>o.id), b: mem.hotRows.slice(15,30).map(o=>o.id) }
// --- eval 4
mem.zoom(799, 1649)
mem.zoom(1476, 2004)
return "zoomed #2, #1"
// --- eval 5
mem.zoom(791, 1050, 44)
mem.zoom(875, 1085, 40)
return "zoomed streak #103/#20 and #37"
// --- eval 6
mem.sweep(0); mem.sweep(1); mem.sweep(2); mem.sweep(3)
return "swept all quarters"
// --- eval 7
return [mem.sweep(0), mem.sweep(1), mem.sweep(2), mem.sweep(3)]
// --- eval 8
const a = await mem.sweep(2)
const b = await mem.sweep(3)
return [a, b]
// --- eval 9
const rows = mem.hotRows
const sky = rows.filter(o => o.sp >= 0.012 && !(o.cy < 1100) && !(o.cx > 1200 && o.cy < 1630) && !o.edge)
return { n: sky.length, sky: sky.map(o=>[o.id,o.cx,o.cy,o.sp,o.snr,o.chroma]) }
// --- eval 10
const sky = mem.hotRows.filter(o => o.sp >= 0.012 && !(o.cy < 1100) && !(o.cx > 1200 && o.cy < 1630) && !o.edge)
mem.sheet(sky.slice(0,15), 44, "sky A")
mem.sheet(sky.slice(15,31), 44, "sky B")
return sky.slice(0,31).map(o=>o.id)
// --- eval 11
const guesses = [[620,1656],[648,1781],[960,1793],[874,1855],[672,1888],[800,1952],[630,2010],[1352,1888],[1440,1975],[1060,1700],[700,1600]]
const out = []
for (const g of guesses) { const p = await mem.probe(g[0], g[1]); out.push([g[0], g[1], p.cx, p.cy, p.sp, p.snr]) }
const a = await mem.sweep(0)
const b = await mem.sweep(1)
return { probes: out, looks: [a, b] }
// --- eval 12
const h2 = await mem.hot(0.5)
const sky2 = mem.hotRows.filter(o => !(o.cy < 1100) && !(o.cx > 1200 && o.cy < 1630) && !o.edge)
const mid = sky2.filter(o => o.sp >= 0.008)
mem.sheet(mid.slice(0, 15), 64, "sky mid A")
return { total: mem.hotRows.length, sky: sky2.length, mid: mid.length, ids: mid.slice(0,15).map(o=>[o.id,o.cx,o.cy,o.sp,o.snr]) }
// --- eval 13
mem.zoom(887, 1861)
mem.zoom(624, 1790)
const items = [[622,1664],[743,1966],[1320,1878],[924,1411],[844,1943],[915,1401],[835,1899],[1064,1788],[729,1167],[1166,1893]][0]
mem.sheet([[622,1664],[743,1966],[1320,1878],[924,1411],[844,1943],[915,1401],[835,1899],[1064,1788],[729,1167],[1166,1893]], 90, "mid candidates")
return "done"
// --- eval 14
mem.sheet([[622,1664],[1320,1878],[844,1943],[835,1899]], 120, "mid1")
mem.sheet([[1064,1788],[743,1966],[1166,1893],[924,1411]], 120, "mid2")
return "mid sheets"
// --- eval 15
mem.sheet([[622,1664],[1320,1878],[844,1943]], 100, "m1")
mem.sheet([[835,1899],[1064,1788],[743,1966]], 100, "m2")
mem.sheet([[1166,1893],[924,1411],[915,1401]], 100, "m3")
return "3 sheets at 4x"
// --- eval 16
mem.sheet([[729,1167],[770,1725],[958,1772]], 100, "n1")
mem.sheet([[585,1874],[859,2001],[779,1951]], 100, "n2")
mem.sheet([[1159,2004],[917,1866],[951,1595]], 100, "n3")
return "n sheets"
// --- eval 17
const e = mem.hotRows.filter(o=>o.edge && o.sp>=0.015)
return { n: e.length, rows: e.slice(0,12).map(o=>[o.id,o.cx,o.cy,o.sp,o.snr]) }
// --- eval 18
mem.sheet([[512.5,1260],[1535.5,1263.5],[892.5,2047.5]], 100, "edges")
const pr = []
for (const c of [[799,1649],[1476,2004],[887,1861],[624,1790],[622,1664],[924,1411],[915,1401],[1064,1788],[743,1966],[770,1725],[729,1167],[1320,1878],[844,1943],[835,1899]]) {
  const p = await mem.probe(c[0], c[1]); pr.push([c[0],c[1],p.cx,p.cy,p.r,p.sp,p.snr,p.chroma,p.cover])
}
return { probes: pr }
