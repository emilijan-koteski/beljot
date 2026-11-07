import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { createRequire } from "node:module";

const here = dirname(fileURLToPath(import.meta.url));
const brand = join(here, "..");
const require = createRequire(join(here, "..", "..", "..", "client", "package.json"));
const { chromium } = require("playwright");

const jobs = JSON.parse(readFileSync(join(here, "build", "png_jobs.json"), "utf8"));
const pdfs = [
  ["beljot-chip.svg", 100],
  ["beljot-lockup-on-dark.svg", 180, "#0b1f15"],
  ["beljot-lockup-on-light.svg", 180],
  ["beljot-chip-black.svg", 100],
  ["beljot-chip-gold.svg", 100],
];

const browser = await chromium.launch();
for (const job of jobs) {
  const page = await browser.newPage({ viewport: { width: job.size, height: job.size } });
  const svg = readFileSync(job.svg, "utf8").replace("<svg ", `<svg width="${job.size}" height="${job.size}" `);
  await page.setContent(`<html><body style="margin:0;background:transparent">${svg}</body></html>`);
  await page.screenshot({ path: job.png, omitBackground: true, clip: { x: 0, y: 0, width: job.size, height: job.size } });
  await page.close();
}
for (const [src, widthMm, bg = "transparent"] of pdfs) {
  const svg = readFileSync(join(brand, src), "utf8");
  const [, , w, h] = svg.match(/viewBox="([^"]+)"/)[1].split(" ").map(Number);
  const heightMm = Math.round(((widthMm * h) / w) * 100) / 100;
  const page = await browser.newPage();
  await page.setContent(
    `<html><head><style>@page{size:${widthMm}mm ${heightMm}mm;margin:0}html,body{margin:0;background:${bg}}svg{display:block;width:${widthMm}mm;height:${heightMm}mm}</style></head><body>${svg}</body></html>`,
  );
  await page.pdf({ path: join(brand, src.replace(".svg", ".pdf")), width: `${widthMm}mm`, height: `${heightMm}mm`, printBackground: true, pageRanges: "1" });
  await page.close();
}
await browser.close();
