// replay of pi annotator on tile 1496ee00c58eed64_1536_1024: 23 evals in its order (GCV-007; helpers via --prelude)
// --- eval 1
mem.tile("1496ee00c58eed64_1536_1024", 1536, 1024)
const v = mem.views()
return { scene: v.scene.look("scene"), speck: v.speck.look("speck"), speckColor: v.speckColor.look("speckColor") }
// --- eval 2
const h = await mem.hot()
mem.hotLook(40)
return { n: h.length, top: h.slice(0, 40) }
// --- eval 3
mem.sheet(mem.hotRows.slice(0, 15), 40, "hot 0-14")
mem.sheet(mem.hotRows.slice(15, 30), 40, "hot 15-29")
return mem.hotRows.slice(0, 30).map(o => o.id + "@" + o.cx + "," + o.cy).join(" ")
// --- eval 4
mem.sweep(0)
mem.sweep(1)
return "sweeps TL TR"
// --- eval 5
const a = mem.sweep(0); return a
// --- eval 6
const b = mem.sweep(2); return b
// --- eval 7
const c = mem.sweep(3); return c
// --- eval 8
const d = mem.sweep(1); return d
// --- eval 9
const out = []
for (const o of mem.hotRows.slice(0, 30)) {
  const p = await mem.probe(o.cx, o.cy)
  out.push({ id: o.id, x: +p.cx.toFixed(1), y: +p.cy.toFixed(1), snr: +p.snr.toFixed(2), sp: +p.sp.toFixed(3), chroma: p.chroma === null || p.chroma === undefined ? null : +Number(p.chroma).toFixed(2), cover: +Number(p.cover).toFixed(2) })
}
return out
// --- eval 10
const pick = [3,1,7,18,4,112].map(id => mem.hotRows.find(o=>o.id===id))
mem.sheet(pick, 72, "sky A")
const pick2 = [105,27,69,72,40,94].map(id => mem.hotRows.find(o=>o.id===id))
mem.sheet(pick2, 72, "sky B")
return "two sheets"
// --- eval 11
const ids = [30,11,33,81,26,25]
const pick = ids.map(id => mem.hotRows.find(o=>o.id===id))
mem.sheet(pick, 72, "arch C")
const arr = []
for (const o of pick) { const p = await mem.probe(o.cx,o.cy); arr.push({id:o.id, x:+p.cx.toFixed(1), y:+p.cy.toFixed(1), snr:+p.snr.toFixed(2), sp:+p.sp.toFixed(3), chroma: p.chroma==null?null:+Number(p.chroma).toFixed(2), cover:+Number(p.cover).toFixed(2)}) }
return arr
// --- eval 12
mem.zoom(1552, 1732, 40)
return await mem.probe(1552, 1732)
// --- eval 13
mem.sheet(mem.hotRows.slice(30,45), 44, "hot 30-44")
mem.sheet(mem.hotRows.slice(45,60), 44, "hot 45-59")
return mem.hotRows.slice(30,60).map(o=>o.id+"@"+o.cx+","+o.cy+" sp"+o.sp).join(" | ")
// --- eval 14
const d = cv.docs.search("levels percentile stretch"); return d
// --- eval 15
const d = cv.docs.read("views"); return d.slice(0, 4000)
// --- eval 16
const d = cv.docs.read("views"); return d.slice(3800)
// --- eval 17
const L = mem.lin
const dg = L.dog(1.5, 6, { polarity: "bright" })
const c1 = dg.crop([2050, 1030, 510, 1018])
const s = await c1.stats()
return { keys: Object.keys(s), s }
// --- eval 18
const L = mem.lin
const dg = L.dog(1.5, 6, { polarity: "bright" })
const a = dg.crop([2050,1030,510,1018])
const b = dg.crop([1700,1400,560,648])
const sa = await a.stats(); const sb = await b.stats()
a.mul(6/sa.p99).look("right sky dog x" + (6/sa.p99).toFixed(1))
b.mul(6/sb.p99).look("centre sky dog")
return { pa: sa.p99, pb: sb.p99 }
// --- eval 19
const G = mem.lin.toGray("average").gaussian(0.8)
const dg = G.dog(2, 8, { polarity: "bright" })
const a = dg.crop([2050,1030,510,1018])
const b = dg.crop([1700,1400,560,648])
const sa = await a.stats(); const sb = await b.stats()
a.mul(1.2/sa.p99).look("right sky gray dog")
b.mul(1.2/sb.p99).look("centre sky gray dog")
return { pa: sa.p99, pb: sb.p99 }
// --- eval 20
const h2 = await mem.hot(0.6)
const sky = h2.filter(o => o.sp >= 0.10 && !(o.cx < 1900 && o.cy < 1400))
const pa = await mem.probe(2095,1757); const pb = await mem.probe(1845,1835)
return { n: h2.length, sky: sky, pa: pa, pb: pb }
// --- eval 21
const ids=[403,618,358,2136,1211,507,2649,191]
const pick = ids.map(id=>mem.hotRows.find(o=>o.id===id)).filter(Boolean)
mem.sheet(pick, 64, "roof/edge D")
return pick.map(o=>o.id+"@"+o.cx+","+o.cy+" sp"+o.sp+" snr"+o.snr)
// --- eval 22
const ids=[3,1,7,18,94,69,40,72]
const pick = ids.map(id=>mem.hotRows.find(o=>o.id===id)).filter(Boolean)
mem.sheet(pick, 88, "final check")
const out=[]
for (const o of pick) { const p = await mem.probe(o.cx,o.cy); const q = await mem.probeR(o.cx,o.cy,3); out.push({id:o.id,x:+p.cx.toFixed(1),y:+p.cy.toFixed(1),snr:+p.snr.toFixed(2),sp:+p.sp.toFixed(3),chroma:p.chroma==null?null:+Number(p.chroma).toFixed(2),sp3:+q.sp.toFixed(3)}) }
return out
// --- eval 23
const pts=[[1935,1900],[1932,1890],[1940,1918],[2300,1882],[2296,1876],[1755,1879],[2363,1568],[1942,1844]]
const out=[]
for (const [x,y] of pts){ const p=await mem.probe(x,y); out.push({x,y,cx:+p.cx.toFixed(1),cy:+p.cy.toFixed(1),snr:+p.snr.toFixed(2),sp:+p.sp.toFixed(3)}) }
return out
