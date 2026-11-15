# Usage evidence

3042 agent evals (Sonnet 335, pi 2707) from 171 agents; helper pastes excluded.

| pattern | occurrences | evals using it | share of evals |
| --- | --- | --- | --- |
| cv.open | 0 | 0 | 0 % |
| mem.tile (1024² gold tile) | 161 | 161 | 5 % |
| crop of a computed image | 50 | 43 | 1 % |
| crop of anything | 353 | 190 | 6 % |
| look with zoom | 268 | 140 | 5 % |
| look with box/ruler | 186 | 100 | 3 % |
| mem.zoom helper | 1528 | 849 | 28 % |
| cv.sheet / mem.sheet | 1217 | 763 | 25 % |
| mem.probe/probeR | 436 | 312 | 10 % |
| measure | 0 | 0 | 0 % |
| peaks | 3 | 3 | 0 % |
| view("speck") | 287 | 276 | 9 % |
| gaussian | 9 | 9 | 0 % |
| dog | 3 | 3 | 0 % |
| erode/dilate | 12 | 4 | 0 % |
| stats | 24 | 17 | 1 % |

## Blur sigmas

- 1.5: 4
- ('dog', 1.5, 6.0): 2
- 1.0: 2
- 0.8: 1
- ('dog', 2.0, 8.0): 1
- 2.5: 1
- 2.0: 1

## What cv.open was called with (top 10)

