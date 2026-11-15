// replay of pi annotator on tile 1ce57fe44ccc51bc_512_1024: 25 evals in its order (GCV-007; helpers via --prelude)
// --- eval 1
mem.tile("1ce57fe44ccc51bc_512_1024", 512, 1024)
const v = mem.views()
return { scene: v.scene.look("scene"), speck: v.speck.look("speck"), speckColor: v.speckColor.look("speckColor") }
// --- eval 2
const h = await mem.hot()
mem.hotLook(40)
return { n: h.length, top: h.slice(0, 40) }
// --- eval 3
const rows = mem.hotRows
const strong = rows.filter(o => o.sp >= 0.15)
const dark = strong.filter(o => o.cx < 1150)
const mid = strong.filter(o => o.cx >= 1150 && o.cx < 1260)
return { total: rows.length, sp15: strong.length, darkLt1150: dark.length, mid: mid.length,
  darkRows: dark.slice(0,30), midRows: mid.slice(0,12) }
// --- eval 4
const dark = mem.hotRows.filter(o => o.sp >= 0.15 && o.cx < 1150)
mem.sheet(dark.slice(0, 15), 44, "dark 0-14")
mem.sheet(dark.slice(15, 24), 44, "dark 15-23")
return dark.slice(0,24).map(o=>o.id).join(",")
// --- eval 5
mem.zoom(684, 1431)
mem.zoom(700, 1492)
return "A"
// --- eval 6
await mem.sweep(0); await mem.sweep(2); return "sweeps 0,2"
// --- eval 7
mem.zoom(740, 1760, 60)
const near = mem.hotRows.filter(o => Math.abs(o.cx-740)<20 && o.cy>1690 && o.cy<1830)
return near
// --- eval 8
mem.zoom(740, 1760, 110)
const ps = []
for (const y of [1660,1680,1700,1800,1820,1840,1860]) ps.push(await mem.probe(740,y))
return ps
// --- eval 9
await mem.sweep(1); await mem.sweep(3); return "sweeps 1,3"
// --- eval 10
const d2 = mem.hotRows.filter(o => o.sp >= 0.08 && o.sp < 0.15 && o.cx < 1150 && o.cx>640)
return { n: d2.length, rows: d2.slice(0, 40) }
// --- eval 11
mem.zoom(737,1162); mem.zoom(1132,1750); return "B"
// --- eval 12
mem.scz = (x,y,half=24) => mem.views().speckColor.crop([x-half,y-half,2*half,2*half]).look("sc "+x+","+y)
mem.scz(737,1162)
mem.scz(700,1492)
mem.scz(655,1438)
return "scz"
// --- eval 13
const s = cv.help("crop")
const ls = cv.ops ? cv.ops() : "none"
return { s, ls }
// --- eval 14
return cv.help("zoom")
// --- eval 15
return String(mem.zoom)
// --- eval 16
mem.scz2 = (x,y,half=24,z=8) => mem.img.view("speckColor").crop([x-half,y-half,2*half,2*half]).look({label:"sc "+x+","+y, zoom:z, ruler:true})
mem.img.view("speckColor").crop([600,1400,220,120]).look({label:"scS", zoom:4})
mem.scz2(737,1162,16,10)
mem.scz2(700,1492,16,10)
return "ok"
// --- eval 17
mem.img.crop([640,1024,400,500]).levels({auto:true}).look({label:"darkA", zoom:2})
mem.img.crop([640,1524,400,524]).levels({auto:true}).look({label:"darkB", zoom:2})
return "darkA/B"
// --- eval 18
const mid = mem.hotRows.filter(o => o.sp >= 0.15 && o.cx >= 1150 && o.cx < 1270)
mem.sheet(mid.slice(0,15), 46, "mid0")
mem.sheet(mid.slice(15,31), 46, "mid1")
return mid.length
// --- eval 19
mem.img.view("speck").crop([640,1075,400,110]).look({label:"bandSpeck", zoom:3})
const tr = mem.hotRows.filter(o => o.sp >= 0.15 && o.cx >= 1270)
mem.sheet(tr.slice(0,15), 46, "tree0")
return tr.length
// --- eval 20
mem.img.crop([712,1690,56,56]).levels({auto:true}).look({label:"scrTop", zoom:7, ruler:true})
mem.img.crop([712,1806,56,56]).levels({auto:true}).look({label:"scrBot", zoom:7, ruler:true})
return "scr ends"
// --- eval 21
const ps=[]
for (const [x,y] of [[736,1730],[738,1755],[739,1780],[740,1805],[740,1822],[741,1840]]) ps.push(await mem.probe(x,y))
mem.img.view("speck").crop([1290,1420,220,280]).look({label:"ornSpeck", zoom:3})
mem.img.crop([1290,1420,220,280]).levels({auto:true}).look({label:"ornScene", zoom:3})
return ps
// --- eval 22
mem.img.view("speck").crop([1085,1085,70,100]).look({label:"thr1", zoom:5, ruler:true})
mem.img.crop([1085,1085,70,100]).levels({auto:true}).look({label:"thr1s", zoom:5, ruler:true})
return "thr"
// --- eval 23
const all = await mem.hot(0.5)
const dz = all.filter(o => o.cx>620 && o.cx<1160 && o.cy>1024 && o.cy<2050)
const dz2 = dz.filter(o => o.sp>=0.06)
return { nAll: all.length, nDark: dz.length, cand: dz2.slice(0,30) }
// --- eval 24
const ids = [40,24,882,260,469,993,1149,154,2468]
const sel = mem.hotRows.filter(o => ids.includes(o.id))
mem.sheet(sel, 50, "darkFaint")
return sel.map(o=>o.id).join(",")
// --- eval 25
const ids2=[2418,5251,756,5007,3524,714,3414,707,119]
const sel2 = mem.hotRows.filter(o=>ids2.includes(o.id))
mem.sheet(sel2, 50, "darkFaint2")
return sel2.map(o=>o.id+" "+o.cx.toFixed(0)+","+o.cy.toFixed(0)+" sp"+o.sp+" snr"+o.snr+" ch"+o.chroma).join(" | ")
