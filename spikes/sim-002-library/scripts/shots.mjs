import { chromium } from "playwright-core";
import { mkdtempSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { serve } from "./serve.mjs";
const root = fileURLToPath(new URL("..", import.meta.url));
const server = await serve(`${root}dist`, 5195);
const ctx = await chromium.launchPersistentContext(mkdtempSync(`${root}bench/out/p-`), { executablePath: "C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe", headless: false, viewport: { width: 1500, height: 900 } });
const page = ctx.pages()[0];
const errs = [];
page.on("pageerror", (e) => errs.push(String(e)));
page.on("console", (m) => m.type() === "error" && errs.push(m.text()));
await page.goto("http://127.0.0.1:5195/");
await page.waitForFunction(() => !!window.__api, null, { timeout: 60000 });
await page.waitForTimeout(1500);
await page.screenshot({ path: `${root}results/1-groups.png` });
const timings = {};
for (const q of ["ecs", "redis", "load balancer", "postgres", "queue", "k8s", "kafka", "nat", "fargate service", "xyzzy"]) {
  await page.fill(".search", q);
  await page.waitForTimeout(150);
  timings[q] = await page.evaluate(() => ({ ms: window.__lastSearchMs, first: [...document.querySelectorAll(".row .name")].slice(0, 4).map((n) => n.textContent), groups: [...document.querySelectorAll(".gh.static")].map((g) => g.firstChild.textContent) }));
  if (q === "ecs" || q === "redis") await page.screenshot({ path: `${root}results/2-search-${q}.png` });
}
await page.fill(".search", "");
for (const id of ["src.users", "aws.cloudfront", "aws.alb", "aws.ecs-service-fargate", "aws.elasticache-valkey", "aws.aurora-postgres", "aws.sqs", "aws.lambda"]) {
  await page.evaluate((id) => window.__place(id), id);
  await page.waitForTimeout(150);
}
await page.evaluate(() => window.__place("aws.ecs-cluster-fargate"));
await page.waitForTimeout(800);
await page.screenshot({ path: `${root}results/3-placed.png` });
await page.selectOption(".lib-head select", "azure");
await page.waitForTimeout(500);
await page.screenshot({ path: `${root}results/4-azure.png` });
writeFileSync(`${root}results/search.json`, JSON.stringify(timings, null, 1));
console.log(JSON.stringify(timings, null, 1));
console.log("errors", errs.slice(0, 5));
await ctx.close(); server.close();
