#!/usr/bin/env python3
"""Count API feature usage and error codes in pi agent runs (GCV-004 evidence).

usage: 01-pi-api-usage.py PI_RUNS_DIR
  PI_RUNS_DIR  directory of run-*/calls.txt (made by CVJS-001 scripts/07-pi-events-summary.py), e.g.
               ~/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/08/CVJS-001--…/various/pi-runs
Prints: number of calls; occurrences of each feature pattern in the code the agent wrote; `{ space }`
values and the ops they were passed to; error codes in results with example messages; option values
that were unawaited nodes (value lifting).
"""
import glob, os, re, sys
from collections import Counter

root = sys.argv[1]
code, res = [], []
for f in sorted(glob.glob(os.path.join(root, "*", "calls.txt"))):
    run = os.path.basename(os.path.dirname(f))
    t = open(f).read()
    for m in re.finditer(r"--- code:\n(.*?)\n--- result:\n(.*?)(?=\n===== call|\Z)", t, re.S):
        code.append((run, m.group(1)))
        res.append(m.group(2))
C = "\n".join(c for _, c in code)
print("runs", len({r for r, _ in code}), "calls", len(code))
pats = {
    "P.* params": r"\bP\.\w+", "cv.evaluate": r"cv\.evaluate", "await": r"\bawait\b",
    "{ space: }": r"\bspace\s*:", ".look(": r"\.look\(", "show(": r"\bshow\(", "mem.": r"\bmem\.",
    "cv.json/cv.csv": r"cv\.(json|csv)\(", ".save(": r"\.save\(", "as: (named image)": r"\bas\s*:",
    "encoding:": r"\bencoding\s*:", ".overlay(": r"\.overlay\(", ".crop(": r"\.crop\(", ".view(": r"\.view\(",
    "cv.sheet(": r"cv\.sheet\(", ".where(": r"\.where\(", ".sortBy(": r"\.sortBy\(", ".top(": r"\.top\(",
    ".filter( (JS)": r"\.filter\(", ".blend(": r"\.blend\(", ".channel(": r"\.channel\(", "mask.gradient": r"mask\.gradient",
}
for k, p in pats.items():
    print(f"  {k:24s} {len(re.findall(p, C))}")
ops = Counter()
for _, c in code:
    for m in re.finditer(r'\.([\w.]+)\(\s*(?:[^()]*?,\s*)?\{[^{}]*\bspace\s*:\s*"(\w+)"', c):
        ops[(m.group(1), m.group(2))] += 1
print("{ space } by op:", ops.most_common(15))
errs = Counter(re.findall(r"E_[A-Z_]+", " ".join(res)))
print("error codes:", errs.most_common(15))
for ec in ["E_UNKNOWN_METHOD", "E_ACCESS", "E_ARG_TYPE", "E_WRONG_KIND", "E_LAZY_COMPARE"]:
    ex = [m.group(1) for r in res if ec in r for m in [re.search(r"message: (.{0,140})", r)] if m]
    print(f"  {ec} e.g.:", ex[:5])
print("E_ACCESS by run:", Counter(r for (r, _), x in zip(code, res) if "E_ACCESS" in x).most_common())
print("value lifting (unawaited node as an option value):")
for r, c in code:
    for m in re.finditer(r"\{[^{}]*:\s*\w+\.stats\(\)\.\w+[^{}]*\}", c):
        print("  ", r, m.group(0)[:100])
