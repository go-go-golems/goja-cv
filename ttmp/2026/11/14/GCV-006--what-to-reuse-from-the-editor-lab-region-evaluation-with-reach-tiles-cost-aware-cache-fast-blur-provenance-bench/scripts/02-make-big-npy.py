#!/usr/bin/env python3
"""02-make-big-npy: a synthetic 6000 x 4000 x 3 float32 scan (noise around 0.3, 400 bright 3 x 3 specks) for timing
goja-cv on a 24-megapixel image. Usage: python3 02-make-big-npy.py OUT_DIR  (writes OUT_DIR/big.npy, 288 MB)."""
import sys
import numpy as np

out = sys.argv[1] if len(sys.argv) > 1 else "."
rng = np.random.default_rng(1)
a = (0.3 + 0.05 * rng.standard_normal((4000, 6000, 3))).astype(np.float32).clip(0, 1)
for _ in range(400):
    y, x = rng.integers(20, 3980), rng.integers(20, 5980)
    a[y - 1:y + 2, x - 1:x + 2] += 0.4
np.save(f"{out}/big.npy", a.clip(0, 1))
