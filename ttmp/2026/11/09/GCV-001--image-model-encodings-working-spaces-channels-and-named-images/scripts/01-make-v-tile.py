#!/usr/bin/env python3
"""Re-export a DUST-002 cv tile (which stores sRGB_encode(v)) as v itself.

usage: 01-make-v-tile.py TILE.tif OUT_v.tif

The DUST-002 export stored sRGB_encode(v) so that goja-cv's old decode recovered
v. With GCV-001 the tile can store v and be opened with { encoding: "linear" }.
This writes that file (16-bit RGB, uncompressed) and prints the numbers the
GCV-001 pi questions ask for, computed independently of goja-cv.
"""
import sys

import numpy as np
import tifffile


def srgb_decode(e):
    return np.where(e <= 0.04045, e / 12.92, ((e + 0.055) / 1.055) ** 2.4)


def main():
    src, out = sys.argv[1], sys.argv[2]
    code = tifffile.imread(src).astype(np.float64)  # H, W, 3
    e = code / 65535
    v = srgb_decode(e)
    vq = np.round(np.clip(v, 0, 1) * 65535).astype(np.uint16)
    tifffile.imwrite(out, vq, photometric="rgb")
    vf = vq.astype(np.float64) / 65535
    y = 0.2126 * vf[..., 0] + 0.7152 * vf[..., 1] + 0.0722 * vf[..., 2]
    print(f"v tile: {out} {vq.shape}")
    print(f"Q1 file units (Rec.709 luminance of v): p01 {np.quantile(y, 0.01):.5f} p99 {np.quantile(y, 0.99):.5f}")
    print(f"Q2 largest |stored(ref) - stored(v)| in file units: {np.abs(e - vf).max():.5f}")


if __name__ == "__main__":
    main()
