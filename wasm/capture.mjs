// Loads a build in headless Chromium, checks that it starts and that triage
// survives a reload, and writes screenshots.
//
// Usage: node wasm/capture.mjs <dir> <outdir> [query]
//
// query is added to the page URL, such as "demo". Needs playwright-core.
// PLAYWRIGHT_CORE names the module when node cannot find it, and CHROMIUM
// names a browser binary when Playwright's own is not installed.
import { spawn } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const dir = path.resolve(process.argv[2] || 'dist');
const outdir = path.resolve(process.argv[3] || 'shots');
const query = process.argv[4] ? '?' + process.argv[4] : '';
const port = 8900 + Math.floor(Math.random() * 90);
fs.mkdirSync(outdir, { recursive: true });

const mod = process.env.PLAYWRIGHT_CORE;
const { chromium } = await import(mod ? pathToFileURL(path.resolve(mod)).href : 'playwright-core');
const server = spawn(process.execPath, [path.join(here, 'serve.mjs'), dir, String(port)], { stdio: ['ignore', 'pipe', 'inherit'] });
await new Promise((resolve) => server.stdout.once('data', resolve));

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
let browser;
let failed = false;
try {
  browser = await chromium.launch({ executablePath: process.env.CHROMIUM || undefined });
  const context = await browser.newContext({ viewport: { width: 1600, height: 960 }, deviceScaleFactor: 2 });
  const page = await context.newPage();
  const errors = [];
  page.on('pageerror', (e) => errors.push('pageerror: ' + e.message));
  page.on('console', (m) => { if (m.type() === 'error') errors.push('console: ' + m.text()); });

  const url = `http://127.0.0.1:${port}/${query}`;
  const t0 = Date.now();
  await page.goto(url);
  await page.waitForFunction(() => window.mentionsFirstFrame === true, null, { timeout: 60000 });
  console.log(`first frame after ${Date.now() - t0} ms`);
  await sleep(1600); // the intro animation
  await page.screenshot({ path: path.join(outdir, 'browser-start.png') });

  // Feature the second row, then search.
  await page.keyboard.press('j');
  await sleep(200);
  await page.keyboard.press('f');
  await sleep(600);
  await page.screenshot({ path: path.join(outdir, 'browser-featured.png') });
  const saved = await page.evaluate(() => {
    const out = {};
    for (let i = 0; i < localStorage.length; i++) out[localStorage.key(i)] = localStorage.getItem(localStorage.key(i));
    return out;
  });
  console.log('localStorage keys:', Object.keys(saved).join(', ') || '(none)');

  await page.keyboard.press('/');
  for (const ch of 'video') { await page.keyboard.press(ch); await sleep(60); }
  await sleep(500);
  await page.screenshot({ path: path.join(outdir, 'browser-search.png') });
  await page.keyboard.press('Escape');

  // A new visit keeps the triage when the page has a feed. The Featured
  // view shows it.
  await page.close();
  const again = await context.newPage();
  again.on('pageerror', (e) => errors.push('pageerror: ' + e.message));
  await again.goto(url);
  await again.waitForFunction(() => window.mentionsFirstFrame === true, null, { timeout: 60000 });
  await sleep(1600);
  const kept = await again.evaluate(() => {
    for (let i = 0; i < localStorage.length; i++) {
      const k = localStorage.key(i);
      if (k.startsWith('mentions-state:')) {
        const items = JSON.parse(localStorage.getItem(k)).items || {};
        return Object.values(items).filter((v) => v.state === 'featured').length;
      }
    }
    return 0;
  });
  console.log('after reload, featured items in localStorage:', kept);
  await again.keyboard.press('v');
  await again.keyboard.press('v');
  await sleep(400);
  await again.screenshot({ path: path.join(outdir, 'browser-reload.png') });

  // A phone-sized screen.
  const phone = await browser.newPage({ viewport: { width: 390, height: 844 }, deviceScaleFactor: 3 });
  phone.on('pageerror', (e) => errors.push('phone pageerror: ' + e.message));
  await phone.goto(url);
  await phone.waitForFunction(() => window.mentionsFirstFrame === true, null, { timeout: 60000 });
  await sleep(1600);
  await phone.screenshot({ path: path.join(outdir, 'browser-phone.png') });

  if (errors.length) {
    failed = true;
    console.error(errors.join('\n'));
  }
  console.log('wrote', outdir);
} catch (err) {
  failed = true;
  console.error(err);
} finally {
  if (browser) await browser.close();
  server.kill();
}
process.exit(failed ? 1 : 0);
