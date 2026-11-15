// replay of pi annotator on tile 69345e1ba20c5694_2048_3072: 60 evals in its order (GCV-007; helpers via --prelude)
// --- eval 1
mem.tile("69345e1ba20c5694_2048_3072", 2048, 3072)
const v = mem.views()
return { help: cv.help ? "ok" : "no", scene: v.scene.look("scene"), speck: v.speck.look("speck"), speckColor: v.speckColor.look("speckColor") }
// --- eval 2
const h = await mem.hot()
const look = mem.hotLook(40)
return { n: h.length, top: h.slice(0, 40) }
// --- eval 3
mem.sheet(mem.hotRows.slice(0, 15), 40, "hot 0-14")
mem.sheet(mem.hotRows.slice(15, 30), 40, "hot 15-29")
return mem.hotRows.slice(0, 30).map(o => o.id).join(",")
// --- eval 4
mem.zoom(2533, 3190)
mem.zoom(2521, 3231)
return "z"
// --- eval 5
mem.zoom(2457, 3306)
mem.zoom(2854, 3655)
return "z"
// --- eval 6
mem.zoom(2705, 3680)
mem.zoom(3046, 3879)
return "z"
// --- eval 7
mem.zoom(2548, 3182)
mem.zoom(3029, 3511)
return "z"
// --- eval 8
mem.sheet(mem.hotRows.slice(30, 45), 90, "hot 30-44")
return mem.hotRows.slice(30,45).map(o=>o.id+":"+o.cx.toFixed(0)+","+o.cy.toFixed(0)+" sp"+o.sp.toFixed(2)).join(" | ")
// --- eval 9
mem.sheet(mem.hotRows.slice(30, 45), 70, "hot 30-44")
return mem.hotRows.slice(30,45).map(o=>o.id+":"+o.cx.toFixed(0)+","+o.cy.toFixed(0)+" sp"+o.sp.toFixed(2)+" snr"+o.snr.toFixed(1)).join(" | ")
// --- eval 10
mem.sheet(mem.hotRows.slice(30, 40), 60, "hot 30-39")
return mem.hotRows.slice(30,40).map(o=>o.id+":"+o.cx.toFixed(0)+","+o.cy.toFixed(0)+" sp"+o.sp.toFixed(2)+" snr"+o.snr.toFixed(1)).join(" | ")
// --- eval 11
const n15 = mem.hotRows.filter(o=>o.sp>=0.15).length
const n12 = mem.hotRows.filter(o=>o.sp>=0.12).length
return {n15, n12, total: mem.hotRows.length, ids: mem.hotRows.filter(o=>o.sp>=0.15).map(o=>o.id).join(",")}
// --- eval 12
mem.sheet(mem.hotRows.slice(15, 25), 60, "hot 15-24")
return mem.hotRows.slice(15,25).map(o=>o.id+":"+o.cx.toFixed(0)+","+o.cy.toFixed(0)+" sp"+o.sp.toFixed(2)).join(" | ")
// --- eval 13
mem.zoom(2163, 3400)
mem.zoom(2900, 3300)
return "z"
// --- eval 14
mem.sweep(0)
return "q0"
// --- eval 15
const l = mem.sweep(0)
return { l }
// --- eval 16
const a = await mem.sweep(0)
return { a }
// --- eval 17
return await cv.help("crop")
// --- eval 18
const sp = mem.img.view("speck")
const sc = mem.img.view("scene")
const s1 = sp.crop([2370,3072,64,1024])
mem.sheet([[2400,3300]], 40, "x")
return { a: s1.look({ label: "speck strip x2370-2434", zoom: 1 }), b: sc.crop([2370,3072,64,1024]).look({ label: "scene strip x2370-2434", zoom: 1 }) }
// --- eval 19
const sp = mem.img.view("speck"), sc = mem.img.view("scene")
return { a: sp.crop([2330,3250,180,480]).look({ label: "speck thread A", zoom: 2 }),
         b: sc.crop([2330,3250,180,480]).look({ label: "scene thread A", zoom: 2 }) }
// --- eval 20
mem.zoom(2170, 3500)
mem.zoom(2300, 3400)
return "z"
// --- eval 21
const sp = mem.img.view("speck"), sc = mem.img.view("scene")
return { a: sc.crop([2850,3072,120,1024]).look({ label: "scene strip x2850-2970", zoom: 1 }),
         b: sp.crop([2850,3072,120,1024]).look({ label: "speck strip x2850-2970", zoom: 1 }) }
// --- eval 22
const ch = mem.hotRows.filter(o=> o.cx>2870 && o.cx<2960 && o.cy>3080 && o.cy<3720)
return {n: ch.length, rows: ch.map(o=>`${o.id} ${o.cx.toFixed(1)} ${o.cy.toFixed(1)} r${o.r} sp${o.sp.toFixed(2)} snr${o.snr.toFixed(1)}`)}
// --- eval 23
mem.zoom(2920, 3140)
mem.zoom(2910, 3250)
return "z"
// --- eval 24
const r = await cv.docs.search("sample pixel value profile")
return r
// --- eval 25
return await cv.docs.read("measuring")
// --- eval 26
const xs=[], ys=[];
for(let x=2880;x<=2944;x+=1) xs.push(x+0.5);
for(let y=3090;y<=3640;y+=2) ys.push(y+0.5);
const pts=[];
for(const y of ys) for(const x of xs) pts.push([x,y]);
const rows = await mem.img.sample(pts, {radius:0});
const N = rows.length;
let out=[];
let i=0;
for(const y of ys){
  const seg = rows.slice(i, i+xs.length).map(r=>r.meanL);
  i += xs.length;
  const med = seg.slice().sort((a,b)=>a-b)[Math.floor(seg.length/2)];
  let bi=0; for(let k=0;k<seg.length;k++) if(seg[k]>seg[bi]) bi=k;
  out.push([y.toFixed(0), xs[bi].toFixed(1), seg[bi].toFixed(4), med.toFixed(4), (seg[bi]-med).toFixed(4)]);
}
return out.map(r=>r.join(" "))
// --- eval 27
const res=[];
for(let y0=3100; y0<=3620; y0+=20){
  const pts=[];
  for(let y=y0-5;y<=y0+5;y++) for(let x=2885;x<=2945;x++) pts.push([x+0.5,y+0.5]);
  const rows = await mem.img.sample(pts,{radius:0});
  const acc=new Array(61).fill(0);
  let i=0;
  for(let y=y0-5;y<=y0+5;y++){ for(let k=0;k<61;k++){ acc[k]+=rows[i].meanL; i++; } }
  const sm=acc.map((v,k)=>{let s=0,c=0;for(let d=-1;d<=1;d++){const j=k+d;if(j>=0&&j<61){s+=acc[j];c++;}}return s/c;});
  const base = sm.slice().sort((a,b)=>a-b)[Math.floor(61/2)];
  let bi=0; for(let k=0;k<61;k++) if(sm[k]>sm[bi]) bi=k;
  res.push(`${y0} x=${2885+bi} peak=${(sm[bi]/11).toFixed(3)} med=${(base/11).toFixed(3)} d=${((sm[bi]-base)/11).toFixed(3)}`);
}
return res
// --- eval 28
const res=[];
for(let y0=3400; y0<=3720; y0+=20){
  const X0=2850, X1=2970, N=X1-X0+1;
  const pts=[];
  for(let y=y0-5;y<=y0+5;y++) for(let x=X0;x<=X1;x++) pts.push([x+0.5,y+0.5]);
  const rows = await mem.img.sample(pts,{radius:0});
  const acc=new Array(N).fill(0); let i=0;
  for(let y=y0-5;y<=y0+5;y++){ for(let k=0;k<N;k++){ acc[k]+=rows[i].meanL; i++; } }
  const sm=acc.map((v,k)=>{let s=0,c=0;for(let d=-1;d<=1;d++){const j=k+d;if(j>=0&&j<N){s+=acc[j];c++;}}return s/c/11;});
  const base = sm.slice().sort((a,b)=>a-b)[Math.floor(N/2)];
  const idx=[...sm.keys()].sort((a,b)=>sm[b]-sm[a]);
  res.push(`${y0}: `+[0,1,2].map(q=>`x${X0+idx[q]} v${sm[idx[q]].toFixed(3)}`).join("  ")+`  med${base.toFixed(3)}`);
}
return res
// --- eval 29
const res=[];
for(let y0=3080; y0<=3240; y0+=20){
  const X0=2870, X1=2960, N=X1-X0+1;
  const pts=[];
  for(let y=y0-5;y<=y0+5;y++) for(let x=X0;x<=X1;x++) pts.push([x+0.5,y+0.5]);
  const rows = await mem.img.sample(pts,{radius:0});
  const acc=new Array(N).fill(0); let i=0;
  for(let y=y0-5;y<=y0+5;y++){ for(let k=0;k<N;k++){ acc[k]+=rows[i].meanL; i++; } }
  const sm=acc.map((v,k)=>{let s=0,c=0;for(let d=-1;d<=1;d++){const j=k+d;if(j>=0&&j<N){s+=acc[j];c++;}}return s/c/11;});
  const base = sm.slice().sort((a,b)=>a-b)[Math.floor(N/2)];
  const idx=[...sm.keys()].sort((a,b)=>sm[b]-sm[a]);
  res.push(`${y0}: `+[0,1,2,3].map(q=>`x${X0+idx[q]} v${sm[idx[q]].toFixed(3)}`).join("  ")+`  med${base.toFixed(3)}`);
}
return res
// --- eval 30
const sc = mem.img.view("scene"), spv = mem.img.view("speck")
return { a: sc.crop([2860,3160,140,340]).look({label:"scene thread B mid", zoom:3}),
         b: spv.crop([2860,3160,140,340]).look({label:"speck thread B mid", zoom:3}) }
// --- eval 31
const sc = mem.img.view("scene"), spv = mem.img.view("speck"), spc = mem.img.view("speckColor")
return { a: sc.crop([2850,3400,140,240]).look({label:"scene threadB low", zoom:3}),
         b: spc.crop([2860,3160,140,340]).look({label:"speckColor threadB", zoom:3}),
         c: spv.crop([2850,3400,140,240]).look({label:"speck threadB low", zoom:3}) }
// --- eval 32
const res=[];
for(let y0=3300; y0<=3680; y0+=20){
  const X0=2370, X1=2450, N=X1-X0+1;
  const pts=[];
  for(let y=y0-5;y<=y0+5;y++) for(let x=X0;x<=X1;x++) pts.push([x+0.5,y+0.5]);
  const rows = await mem.img.sample(pts,{radius:0});
  const acc=new Array(N).fill(0); let i=0;
  for(let y=y0-5;y<=y0+5;y++){ for(let k=0;k<N;k++){ acc[k]+=rows[i].meanL; i++; } }
  const sm=acc.map((v,k)=>{let s=0,c=0;for(let d=-1;d<=1;d++){const j=k+d;if(j>=0&&j<N){s+=acc[j];c++;}}return s/c/11;});
  const base = sm.slice().sort((a,b)=>a-b)[Math.floor(N/2)];
  const idx=[...sm.keys()].sort((a,b)=>sm[b]-sm[a]);
  res.push(`${y0}: `+[0,1,2].map(q=>`x${X0+idx[q]} v${sm[idx[q]].toFixed(3)}`).join("  ")+`  med${base.toFixed(3)}`);
}
return res
// --- eval 33
const sc = mem.img.view("scene"), spv = mem.img.view("speck")
return { a: sc.crop([2880,3072,192,150]).look({label:"scene topright", zoom:3}),
         b: spv.crop([2880,3072,192,150]).look({label:"speck topright", zoom:3}) }
// --- eval 34
const a = await mem.sweep(1)
return {a}
// --- eval 35
mem.sheet(mem.hotRows.slice(25, 35), 60, "hot 25-34")
return mem.hotRows.slice(25,35).map(o=>o.id+":"+o.cx.toFixed(0)+","+o.cy.toFixed(0)+" sp"+o.sp.toFixed(2)).join(" | ")
// --- eval 36
mem.sheet(mem.hotRows.slice(35, 45), 60, "hot 35-44")
return mem.hotRows.slice(35,45).map(o=>o.id+":"+o.cx.toFixed(0)+","+o.cy.toFixed(0)+" sp"+o.sp.toFixed(2)).join(" | ")
// --- eval 37
mem.sheet(mem.hotRows.slice(45, 55), 60, "hot 45-54")
return mem.hotRows.slice(45,55).map(o=>o.id+":"+o.cx.toFixed(0)+","+o.cy.toFixed(0)+" sp"+o.sp.toFixed(2)).join(" | ")
// --- eval 38
mem.sheet(mem.hotRows.slice(55, 58), 60, "hot 55-57")
return mem.hotRows.slice(55,58).map(o=>o.id+":"+o.cx.toFixed(0)+","+o.cy.toFixed(0)+" sp"+o.sp.toFixed(2)).join(" | ")
// --- eval 39
const a = await mem.sweep(2)
return {a}
// --- eval 40
const q = mem.hotRows.filter(o=>o.cx>=2048&&o.cx<2560&&o.cy>=3584&&o.cy<4096&&o.sp>=0.10)
return q.slice(0,20).map(o=>`${o.id} ${o.cx.toFixed(0)} ${o.cy.toFixed(0)} r${o.r} sp${o.sp.toFixed(2)} snr${o.snr.toFixed(1)}`)
// --- eval 41
mem.sheet([mem.hotRows[{'28':28,'138':138,'211':211,'212':212,'45':45,'34':34,'41':41}[k]-1]||{cx:0,cy:0,id:k} for k in ['28','138','211','212','45','34','41']], 60, "q2 faint")
const a = await mem.sweep(3)
return {a}
// --- eval 42
const ids=[28,138,211,212,45,34,41];
const items = mem.hotRows.filter(o=>ids.includes(o.id));
mem.sheet(items, 60, "q2 faint")
const a = await mem.sweep(3)
return {a, items: items.map(o=>o.id+" "+o.cx.toFixed(0)+","+o.cy.toFixed(0))}
// --- eval 43
const q = mem.hotRows.filter(o=>o.cx>=2560&&o.cx<3072&&o.cy>=3584&&o.cy<4096&&o.sp>=0.10)
return {n:q.length, rows: q.slice(0,16).map(o=>`${o.id} ${o.cx.toFixed(0)} ${o.cy.toFixed(0)} sp${o.sp.toFixed(2)} snr${o.snr.toFixed(1)}`)}
// --- eval 44
mem.zoom(3041, 3836)
mem.zoom(2988, 3595)
return "z"
// --- eval 45
mem.zoom(2975, 3835)
mem.zoom(2609, 3723)
return "z"
// --- eval 46
mem.zoom(2576, 3121)
mem.zoom(2617, 3404)
return "z"
// --- eval 47
const cands = [[2533,3190],[2521,3231],[2457,3306],[3046,3879],[2548,3182],[3029,3511],[2754,3381],[2347,3217],[2797,3597],[2609,3723],[2617,3404],[2975,3835],[2371,3222],[2317,3181],[2389,3381],[2461,3936],[2345,3119],[2988,3872],[2936,3186],[2603,3166],[2577,3175],[2965,3193],[2849,3375],[2897,3278],[2854,3655],[2705,3680],[2963,3661],[2758,3953],[2557,3673],[2949,3580],[2107,3217],[2411,3196],[3037,3774],[3041,3836],[2988,3595],[2576,3121],[2155,3699],[2126,3920],[2280,4083],[2190,4033],[2415,3876],[2917,3585],[2744,3709],[2825,3675],[2975,3585]];
const out=[];
for(const [x,y] of cands){
  const p = await mem.probe(x,y);
  out.push(`${x},${y} -> ${p.cx.toFixed(1)},${p.cy.toFixed(1)} snr${p.snr.toFixed(2)} sp${p.sp.toFixed(2)} chr${(p.chroma==null?"-":p.chroma.toFixed(2))} step${p.step.toFixed(3)} cover${p.cover}`);
}
return out
// --- eval 48
const mid = mem.hotRows.filter(o=>o.sp>=0.12 && o.sp<0.15)
return {n: mid.length, rows: mid.map(o=>`${o.id} ${o.cx.toFixed(0)} ${o.cy.toFixed(0)} sp${o.sp.toFixed(2)} snr${o.snr.toFixed(1)}`)}
// --- eval 49
const ids=[80,172,20,78,90,156,158,418,378,70];
const items = mem.hotRows.filter(o=>ids.includes(o.id));
mem.sheet(items, 60, "sp12-15 a")
return items.map(o=>o.id+":"+o.cx.toFixed(0)+","+o.cy.toFixed(0))
// --- eval 50
const ids=[87,161,73,86,44,298,30,47,72,544];
const items = mem.hotRows.filter(o=>ids.includes(o.id));
mem.sheet(items, 60, "sp12-15 b")
return items.map(o=>o.id+":"+o.cx.toFixed(0)+","+o.cy.toFixed(0))
// --- eval 51
const ids=[210,551,501,592];
const items = mem.hotRows.filter(o=>ids.includes(o.id));
mem.sheet(items, 60, "sp12-15 c")
return items.map(o=>o.id+":"+o.cx.toFixed(0)+","+o.cy.toFixed(0))
// --- eval 52
const sc = mem.img.view("scene"), spv = mem.img.view("speck")
return { a: sc.crop([2048,3072,256,256]).look({label:"scene topleft", zoom:3}),
         b: spv.crop([2048,3072,256,256]).look({label:"speck topleft", zoom:3}) }
// --- eval 53
const sc = mem.img.view("scene"), spv = mem.img.view("speck")
return { a: sc.crop([2120,3600,200,496]).look({label:"scene leftlow", zoom:2}),
         b: spv.crop([2120,3600,200,496]).look({label:"speck leftlow", zoom:2}) }
// --- eval 54
const res=[];
for(let y0=3640; y0<=4080; y0+=40){
  const X0=2100, X1=2240, N=X1-X0+1;
  const pts=[];
  for(let y=y0-8;y<=y0+8;y++) { if(y<3072||y>=4096) continue; for(let x=X0;x<=X1;x++) pts.push([x+0.5,y+0.5]); }
  const nY = 17;
  const rows = await mem.img.sample(pts,{radius:0});
  const acc=new Array(N).fill(0); let i=0;
  for(let y=y0-8;y<=y0+8;y++){ for(let k=0;k<N;k++){ acc[k]+=rows[i].meanL; i++; } }
  const sm=acc.map((v,k)=>{let s=0,c=0;for(let d=-1;d<=1;d++){const j=k+d;if(j>=0&&j<N){s+=acc[j];c++;}}return s/c/nY;});
  const base = sm.slice().sort((a,b)=>a-b)[Math.floor(N/2)];
  const idx=[...sm.keys()].sort((a,b)=>sm[b]-sm[a]);
  res.push(`${y0}: `+[0,1,2].map(q=>`x${X0+idx[q]} v${sm[idx[q]].toFixed(3)}`).join("  ")+`  med${base.toFixed(3)}`);
}
return res
// --- eval 55
mem.zoom(2470, 3712)
return "z"
// --- eval 56
const res=[];
for(let y0=3140; y0<=3340; y0+=20){
  const X0=2370, X1=2430, N=X1-X0+1;
  const pts=[];
  for(let y=y0-5;y<=y0+5;y++) for(let x=X0;x<=X1;x++) pts.push([x+0.5,y+0.5]);
  const rows = await mem.img.sample(pts,{radius:0});
  const acc=new Array(N).fill(0); let i=0;
  for(let y=y0-5;y<=y0+5;y++){ for(let k=0;k<N;k++){ acc[k]+=rows[i].meanL; i++; } }
  const sm=acc.map((v,k)=>{let s=0,c=0;for(let d=-1;d<=1;d++){const j=k+d;if(j>=0&&j<N){s+=acc[j];c++;}}return s/c/11;});
  const base = sm.slice().sort((a,b)=>a-b)[Math.floor(N/2)];
  const idx=[...sm.keys()].sort((a,b)=>sm[b]-sm[a]);
  res.push(`${y0}: `+[0,1,2].map(q=>`x${X0+idx[q]} v${sm[idx[q]].toFixed(3)}`).join("  ")+`  med${base.toFixed(3)}`);
}
return res
// --- eval 57
const sc = mem.img.view("scene"), spv = mem.img.view("speck"), spc = mem.img.view("speckColor")
return { a: sc.crop([2690,3620,180,90]).look({label:"scene car top", zoom:5}),
         b: spv.crop([2690,3620,180,90]).look({label:"speck car top", zoom:5}),
         c: spc.crop([2690,3620,180,90]).look({label:"speckColor car top", zoom:5}) }
// --- eval 58
const sc = mem.img.view("scene"), spv = mem.img.view("speck"), spc = mem.img.view("speckColor")
return { a: sc.crop([2700,3900,160,140]).look({label:"scene car body", zoom:4}),
         b: spv.crop([2700,3900,160,140]).look({label:"speck car body", zoom:4}),
         c: spc.crop([2700,3900,160,140]).look({label:"speckColor car body", zoom:4}) }
// --- eval 59
const sc = mem.img.view("scene"), spv = mem.img.view("speck")
return { a: sc.crop([2540,3072,120,110]).look({label:"scene hook", zoom:5}),
         b: spv.crop([2540,3072,120,110]).look({label:"speck hook", zoom:5}) }
// --- eval 60
const pts=[[2406,3490],[2896,3410],[2190,4035]];
const out=[];
for(const [x,y] of pts){ const p=await mem.probeR(x,y,60); out.push(`${x},${y} snr${p.snr.toFixed(2)} sp${p.sp.toFixed(2)}`); }
return out
