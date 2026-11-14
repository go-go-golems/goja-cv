# Editor lab: the work, 12 to 14 November 2026

The Editor lab is a photo-editing workbench that runs as one HTML page: a tiled, content-cached engine, a stage with
ten tools, an agent and a code editor that drive the same lab API, a benchmark bench, 32 presets and a nine-chapter
textbook on building a fast editing engine in the browser.

- Published page: https://claude.ai/artifact/Xk9E1jXbFtdxc3F8qeL8ph (version 1794688372-d7ad, 14 Nov 2026 20:32 UTC)
- Implementation diary and report: https://claude.ai/artifact/8PsEXsbWo6zaW9X2aqN1Z2 (also `REPORT.pdf` here)

## What is where

| Path | Contents |
| --- | --- |
| `REPORT.pdf` | The implementation diary and report, exported from the doc above |
| `editor-lab/` | The project. `index.html` is the built page, identical to the published version |
| `editor-lab/build.py` | Concatenates `src/` scripts, the book and the WebAssembly kernels into `index.html` |
| `editor-lab/src/` | `engine.js`, `workspace.js`, `app.js`, `dock.js`, `controls.js`, `util.js`, `scenes.js`, `views/` (stage, panels, presets, bench, exports, gpu, wasm, agent, code, book), styles |
| `editor-lab/src/book/` | The nine chapters as Markdown |
| `editor-lab/ds/` | CSS of the Form Skate Workbench, Agent and Stage design systems the page is built on |
| `editor-lab/tools/` | `check_book.js` (chapter checker), `renumber.py`, `embed_figs.py`, `figs.py` and `figs2.py` (computed figures), `kernels.wat` and `wat.js` (WebAssembly kernels) |
| `editor-lab/test/` | Browser tests and measurement scripts (puppeteer-core, headless Chrome) |
| `editor-lab/research/` | `notes.md` and `src/`: the third-party source read for chapters 5 to 9, with `SOURCES.md` |
| `editor-lab/build-output/figures/` | The computed figures as SVG |
| `screenshots/` | Screenshots of the final build, four of them of layer groups |
| `earlier-work/docs/` | PDFs of the two docs written before the workbench |
| `earlier-work/editor-grammar-examples/` | The grammar examples and the script and page that published them |

## Added after the report: layer groups and provenance

- Groups in the engine: a group is an entry in the layer list with its children directly below it. Pass through (the
  default) applies the children to the backdrop and fades their change by opacity and mask; any other blend mode
  isolates them on transparent and blends the result as one layer. `applyGroup` and `Engine.prototype.key` in
  `src/engine.js`; a pass-through group at full opacity shares its top child's cache key.
- Group, Ungroup, drag into a group, fold, duplicate and delete in the Layers panel, each one undo step
  (`restructure` in `src/workspace.js`).
- `meta.source` on every layer records what made it: a preset (id, index), a program or script (file and line), or the
  agent (thread, turn, step). Chips in Layers, a source block in Params, "On the stage" in Presets and a footer link in
  the Code view go both ways. Lab API: `group`, `ungroup`, `moveTo`, `meta`, `source`, `reveal`, `madeBy`.
- Book: new Section 2.5 on groups (figure, quote of `applyGroup`, a run block); Section 6.1's account of the key updated.
- `test/groups.js`: 48 checks (pixels, cache keys, history, provenance, the panel, the book's Try it).

## Rebuild and test

Needs Python 3, Node 18 or later, and Chrome at `/usr/bin/google-chrome` (the tests' path; change it in the test
files for another location).

```bash
cd editor-lab
npm install                    # puppeteer-core
python3 build.py               # writes index.html
node tools/check_book.js       # chapter rules; expect no errors
for t in smoke groups interact masks presets presetcheck code codelayers exports agent agent_call; do node test/$t.js; done
node test/runblocks.js         # the book's runnable programs (BENCH=1 to include the bench one)
node test/quotes.js            # every code quote in the book resolves
python3 tools/figs2.py         # recompute figures into /tmp/figs
python3 tools/embed_figs.py    # replace @@FIG name@@ lines in chapters with /tmp/figs/name.svg
python3 tools/renumber.py      # after editing chapters: renumber and fix references (uses a snapshot in /tmp/book_old)
```

Opening `editor-lab/index.html` in a browser works without a server. The agent, per-user preset storage and confirmed
downloads need the capabilities the published page gets from its host; without them the agent reports that it is
off, presets stay in the browser's local storage and exports are copied to the clipboard.

The tests' error lines are errors they provoke on purpose (an unknown layer, a read-only refusal, a deliberate syntax
error). The agent tests use a stubbed model.
