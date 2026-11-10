"""Prototype of GCV-005 peaks on one DUST-002 tile: candidate counts, snr levels, gold coverage, timings.
Digital disks/annuli (not coverage-weighted), ring [1.5r, 2.5r], snr = step / max(p90 - p10 of ring, 1e-3).
Usage: python3 -I 02-peaks-prototype.py TILE_ID   (GCV-005 guide §3.6; needs numpy, scipy, tifffile)"""
import json, sys, time
import numpy as np
from scipy import ndimage
import tifffile

tid = sys.argv[1]
img = tifffile.imread(f"/Users/manuel/dustdet/cv_tiles/{tid}.tif").astype(np.float64)
if img.ndim == 3 and img.shape[0] in (3, 4) and img.shape[-1] not in (3, 4):
    img = np.moveaxis(img, 0, -1)
img = img[..., :3] / 65535.0
L = 0.2126 * img[..., 0] + 0.7152 * img[..., 1] + 0.0722 * img[..., 2] if img.ndim == 3 else img
h, w = L.shape
ox, oy = [int(v) for v in tid.split("_")[1:3]]
gold = []
for suffix in ("final", "p1"):
    try:
        gold = [p for p in json.load(open(f"/Users/manuel/dustds/gold/{tid}.{suffix}.json"))["points"] if p.get("type") == "dust"]
        break
    except FileNotFoundError:
        pass

def disk(rho):
    R = int(np.floor(rho))
    yy, xx = np.mgrid[-R:R + 1, -R:R + 1]
    return (xx * xx + yy * yy <= rho * rho).astype(np.float64)

ones = np.ones_like(L)
def mean_over(fp):
    s = ndimage.correlate(L, fp, mode="constant", cval=0.0)
    n = ndimage.correlate(ones, fp, mode="constant", cval=0.0)
    return s, n

print(f"tile {tid} {w}x{h}, dust gold points: {len(gold)}")
allpk = []
for r in (2, 4, 8):
    t0 = time.time()
    sc, nc = mean_over(disk(r)); s1, n1 = mean_over(disk(1.5 * r)); s2, n2 = mean_over(disk(2.5 * r))
    S = sc / nc - (s2 - s1) / (n2 - n1)
    t1 = time.time()
    D = ndimage.maximum_filter(S, footprint=disk(r).astype(bool), mode="constant", cval=-np.inf)
    cand = np.argwhere((S > 0) & (S == D))
    t2 = time.time()
    outer = disk(2.5 * r).astype(bool); inner = np.zeros_like(outer); k = (outer.shape[0] - disk(1.5 * r).shape[0]) // 2
    inner[k:k + disk(1.5 * r).shape[0], k:k + disk(1.5 * r).shape[0]] = disk(1.5 * r).astype(bool)
    ring = outer & ~inner
    R = ring.shape[0] // 2
    offs = np.argwhere(ring) - R
    snrs = np.empty(len(cand))
    for i, (y, x) in enumerate(cand):
        yy, xx = offs[:, 0] + y, offs[:, 1] + x
        ok = (yy >= 0) & (yy < h) & (xx >= 0) & (xx < w)
        v = L[yy[ok], xx[ok]]
        spread = np.percentile(v, 90) - np.percentile(v, 10)
        snrs[i] = S[y, x] / max(spread, 1e-3)
    t3 = time.time()
    levels = {k: int((snrs >= k).sum()) for k in (0.5, 1, 1.4, 2)}
    print(f"r={r}: step map {t1-t0:.2f}s, NMS {t2-t1:.2f}s, candidates {len(cand)}, ring snr (python loop) {t3-t2:.1f}s; kept at snr>= {levels}")
    for (y, x), s in zip(cand, snrs):
        allpk.append((s, r, x + 0.5 + ox, y + 0.5 + oy))

for thr in (0.5, 1, 1.4, 2):
    pk = sorted([p for p in allpk if p[0] >= thr], reverse=True)
    kept = []
    for p in pk:
        if all((p[2] - k[2]) ** 2 + (p[3] - k[3]) ** 2 >= (p[1] + k[1]) ** 2 for k in kept[-400:]):
            kept.append(p)
    cov = sum(any((k[2] - g["x"]) ** 2 + (k[3] - g["y"]) ** 2 <= max(g["r"], 3) ** 2 for k in kept) for g in gold)
    near = sum(any((k[2] - g["x"]) ** 2 + (k[3] - g["y"]) ** 2 <= max(g["r"], 3) ** 2 for g in gold) for k in kept)
    print(f"snr>={thr}: peaks {len(kept)}, gold covered {cov}/{len(gold)}, peaks not near gold {len(kept) - near}")
