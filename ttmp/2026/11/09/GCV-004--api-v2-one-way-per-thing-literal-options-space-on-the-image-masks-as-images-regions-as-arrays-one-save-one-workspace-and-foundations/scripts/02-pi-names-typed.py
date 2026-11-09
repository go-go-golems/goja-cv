#!/usr/bin/env python3
"""Names agents typed in the pi runs, and how often they called each method (GCV-004 evidence).

usage: 02-pi-names-typed.py PI_RUNS_DIR [GOJA_CV_DIR]
  PI_RUNS_DIR  directory of run-*/calls.txt (see 01-pi-api-usage.py)
  GOJA_CV_DIR  optional goja-cv checkout: the synonym and redirect keys of its Go tables are listed as
               "typed" (some agent wrote that wrong name) or "never typed" (a candidate for deletion)

Prints:
  1. method/function calls in the agents' code (`.name(` and `cv.name(`), most used first;
  2. wrong names in error results: unknown methods, options, enum values, fields (what agents typed);
  3. with GOJA_CV_DIR: each key of graph.Synonyms, graph.OptionSynonyms, graph.MethodRedirects,
     ArgSpec.ValueSynonyms and ArgSpec.Redirects, marked typed / never typed.
Counts are regex occurrences in the code blocks (uses, not successes).
"""
import glob, os, re, sys
from collections import Counter

root = sys.argv[1]
code, res = [], []
for f in sorted(glob.glob(os.path.join(root, "*", "calls.txt"))):
    t = open(f).read()
    for m in re.finditer(r"--- code:\n(.*?)\n--- result:\n(.*?)(?=\n===== call|\Z)", t, re.S):
        code.append(m.group(1))
        res.append(m.group(2))
C = "\n".join(code)
R = "\n".join(res)
print("calls", len(code))

calls = Counter(re.findall(r"\.([A-Za-z_]\w*)\(", C))
print("\n1. method calls in code (top 120):")
for name, n in calls.most_common(120):
    print(f"  {name:22s} {n}")

wrong = Counter()
pats = {
    "method": r'(?:has no method|no op named|no method) "([^"]+)"',
    "option": r'has no option "([^"]+)"',
    "value": r'= "([^"]+)" is not one of',
    "field": r'(?:have no field|has no field|no field) "([^"]+)"',
    "member": r'cv has no (?:member|function) "([^"]+)"',
}
for kind, p in pats.items():
    for name in re.findall(p, R):
        wrong[(kind, name)] += 1
print("\n2. wrong names in error results:")
for (kind, name), n in sorted(wrong.items(), key=lambda x: (-x[1], x[0])):
    print(f"  {kind:7s} {name:28s} {n}")

if len(sys.argv) > 2:
    src = ""
    for f in glob.glob(os.path.join(sys.argv[2], "pkg", "**", "*.go"), recursive=True):
        if not f.endswith("_test.go"):
            src += open(f).read()
    typed = {name for (_, name) in wrong}
    # a key counts as typed when some error result names it as the wrong name an agent wrote
    keys = Counter()
    for table in ["Synonyms", "OptionSynonyms", "MethodRedirects", "ValueSynonyms", "Redirects", "lookRedirects", "blendRedirects"]:
        for m in re.finditer(r"(?<![A-Za-z])" + table + r"\s*(?:=|:)\s*(?:map\[string\][\w.\[\]]+\s*)?\{", src):
            depth, i = 1, m.end()
            while depth and i < len(src):  # the map literal: up to its closing brace
                depth += {"{": 1, "}": -1}.get(src[i], 0)
                i += 1
            body = src[m.end():i - 1]
            for k in re.findall(r'(?:^|[{,\s])"([A-Za-z_][\w]*)"\s*:', body):
                keys[(table, k)] += 1
    print("\n3. synonym / redirect keys (typed = an agent wrote this wrong name):")
    t, nt = [], []
    for (table, k) in sorted(keys):
        (t if k in typed else nt).append(f"{table}:{k}")
    print("  typed:", ", ".join(t))
    print(f"  never typed ({len(nt)}):", ", ".join(nt))
