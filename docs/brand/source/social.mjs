import { readFileSync, statSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { createRequire } from "node:module";

const here = dirname(fileURLToPath(import.meta.url));
const out = join(here, "..", "social");
const require = createRequire(join(here, "..", "..", "..", "client", "package.json"));
const { chromium } = require("playwright");

const jobs = JSON.parse(readFileSync(join(here, "build", "social", "jobs.json"), "utf8"));
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
  await page.close();
  console.log(`${job.name}.${ext}`, `${Math.round(statSync(path).size / 1024)} KB`);
}
await browser.close();
