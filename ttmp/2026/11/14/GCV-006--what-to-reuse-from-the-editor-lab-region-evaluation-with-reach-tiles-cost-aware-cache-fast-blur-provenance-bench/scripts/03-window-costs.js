// 03-window-costs: time a 64 px window of several results on big.npy (02-make-big-npy.py).
// Run: goja-cv eval --root DIR_WITH_BIG_NPY -e "$(cat 03-window-costs.js)"
const t = async (label, f) => { const t0 = Date.now(); await f(); return label + ": " + (Date.now() - t0) + " ms" }
const img = cv.open("big.npy", { encoding: "linear" })
const out = []
out.push(await t("open+stats (decode)", () => img.stats()))
out.push(await t("crop 64px stats", () => img.crop([3000, 2000, 64, 64]).stats()))
out.push(await t("gaussian s2 crop64", () => img.gaussian(2).crop([3000, 2000, 64, 64]).stats()))
out.push(await t("gaussian s8 crop64", () => img.gaussian(8).crop([3000, 2000, 64, 64]).stats()))
out.push(await t("gaussian s32 crop64", () => img.gaussian(32).crop([3000, 2000, 64, 64]).stats()))
out.push(await t("speck view crop64", () => img.view("speck").crop([3010, 2010, 64, 64]).stats()))
out.push(await t("speck view crop64 again (cached)", () => img.view("speck").crop([100, 100, 64, 64]).stats()))
out.push(await t("peaks r[2,4,8] full", () => img.peaks({ radius: [2, 4, 8], snr: 3 })))
return out
