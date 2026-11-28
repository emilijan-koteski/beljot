import { mkdirSync, readFileSync, statSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { createRequire } from "node:module";

const here = dirname(fileURLToPath(import.meta.url));
const out = join(here, "..", "social");
const require = createRequire(join(here, "..", "..", "..", "client", "package.json"));
const { chromium } = require("playwright");

const jobs = JSON.parse(readFileSync(join(here, "build", "social", "jobs.json"), "utf8"));
/** Cuts a mosaic into tiles named by posting order: part 1 is bottom-right, part 9 top-left. */
async function sliceTiles(page, job) {
  const { width, height, dir } = job.tiles;
  const cols = job.width / width;
  const rows = job.height / height;
  mkdirSync(join(out, dir), { recursive: true });
  for (let row = 0; row < rows; row++) {
    for (let col = 0; col < cols; col++) {
      const order = (rows - 1 - row) * cols + (cols - col);
      const path = join(out, dir, `post-${order}-of-${rows * cols}.jpg`);
      await page.screenshot({ path, type: "jpeg", quality: 92, clip: { x: col * width, y: row * height, width, height } });
    }
  }
}

const browser = await chromium.launch();
for (const job of jobs) {
  const page = await browser.newPage({ viewport: { width: job.width, height: job.height } });
  await page.goto(`file://${job.svg}`);
  await page.waitForLoadState("networkidle");
  await page.waitForTimeout(400);
  const ext = job.type === "jpeg" ? "jpg" : "png";
  const path = join(out, `${job.name}.${ext}`);
  const options = { path, type: job.type, omitBackground: job.type === "png" };
  if (job.type === "jpeg") options.quality = 90;
  await page.screenshot(options);
  if (job.tiles) await sliceTiles(page, job);
  await page.close();
  console.log(`${job.name}.${ext}`, `${Math.round(statSync(path).size / 1024)} KB`);
}
await browser.close();
