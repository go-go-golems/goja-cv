#!/usr/bin/env python3
"""01-usage-evidence: how annotator agents used goja-cv (GCV-007 step 4, evidence for GCV-006 region evaluation).

Reads the Sonnet annotators' evals (DUST-002 various/agent-evals/evals.jsonl) and the pi annotators' tool calls
(~/dustdet/autolabel/pi*/TILE/events.jsonl), and counts in the agents' own code (helpers pasted or preluded excluded):
which images were opened, crops and zoomed/boxed looks of computed images, sheets, blur sigmas, views and helpers.
usage: 01-usage-evidence.py EVALS_JSONL PI_DIR... > report.md"""
import collections, glob, json, os, re, sys

HELPER_MARK = "mem.views = () =>"  # the pasted helper library (84-cv-helpers.js)


def sonnet(path):
    for line in open(path):
        r = json.loads(line)
        code = r["code"]
        if HELPER_MARK in code:
            continue  # the helper paste itself
        yield "sonnet", r["agent"], r["tile"], code


def pi(dirs):
    for d in dirs:
        for ev in glob.glob(os.path.join(d, "*", "events.jsonl")):
            run = os.path.basename(os.path.dirname(ev))
            for line in open(ev):
                try:
                    e = json.loads(line)
                except Exception:
                    continue
                if e.get("type") == "tool_execution_start" and "eval" in str(e.get("toolName")):
                    code = (e.get("args") or {}).get("code", "")
                    if code and HELPER_MARK not in code:
                        yield "pi", os.path.basename(d) + "/" + run, run, code


COMPUTED = r"(?:view|gaussian|dog|levels|exposure|toGray|sub|erode|dilate|highpass|as)\("
PATTERNS = {
    "cv.open": r"cv\.open\(",
    "mem.tile (1024² gold tile)": r"mem\.tile\(",
    "crop of a computed image": COMPUTED + r"[^\n]*?\)\s*\.crop\(",
    "crop of anything": r"\.crop\(",
    "look with zoom": r"look\(\{[^}]*zoom",
    "look with box/ruler": r"look\(\{[^}]*(box|ruler)",
    "mem.zoom helper": r"mem\.zoom\(",
    "cv.sheet / mem.sheet": r"(cv|mem)\.sheet\(",
    "mem.probe/probeR": r"mem\.probeR?\(",
    "measure": r"\.measure\(",
    "peaks": r"\.peaks\(",
    "view(\"speck\")": r"view\(\"speck\"\)|speck:",
    "gaussian": r"\.gaussian\(",
    "dog": r"\.dog\(",
    "erode/dilate": r"\.(erode|dilate)\(",
    "stats": r"\.stats\(",
}


def main():
    evals = list(sonnet(sys.argv[1])) + list(pi(sys.argv[2:]))
    agents = collections.defaultdict(int)
    counts = collections.Counter()
    evals_with = collections.Counter()
    sigmas = collections.Counter()
    opened = collections.Counter()
    for src, agent, tile, code in evals:
        agents[(src, agent)] += 1
        for name, pat in PATTERNS.items():
            k = len(re.findall(pat, code))
            counts[name] += k
            evals_with[name] += 1 if k else 0
        for m in re.finditer(r"\.gaussian\(\s*([0-9.]+)", code):
            sigmas[float(m.group(1))] += 1
        for m in re.finditer(r"\.dog\(\s*([0-9.]+)\s*,\s*([0-9.]+)", code):
            sigmas[("dog", float(m.group(1)), float(m.group(2)))] += 1
        for m in re.finditer(r"cv\.open\(\s*([^,)]+)", code):
            opened[m.group(1).strip()[:60]] += 1
    n_s = sum(1 for e in evals if e[0] == "sonnet")
    n_p = len(evals) - n_s
    print(f"# Usage evidence\n\n{len(evals)} agent evals (Sonnet {n_s}, pi {n_p}) from "
          f"{len(agents)} agents; helper pastes excluded.\n")
    print("| pattern | occurrences | evals using it | share of evals |\n| --- | --- | --- | --- |")
    for name in PATTERNS:
        print(f"| {name} | {counts[name]} | {evals_with[name]} | {100 * evals_with[name] / max(1, len(evals)):.0f} % |")
    print("\n## Blur sigmas\n")
    for k, v in sorted(sigmas.items(), key=lambda kv: -kv[1]):
        print(f"- {k}: {v}")
    print("\n## What cv.open was called with (top 10)\n")
    for k, v in opened.most_common(10):
        print(f"- `{k}`: {v}")


if __name__ == "__main__":
    main()
