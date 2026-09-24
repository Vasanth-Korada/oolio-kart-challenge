// Exports every page of a .drawio file to a cropped PNG, for embedding in the
// README (GitHub can't render .drawio). Each page is opened in the public
// viewer.diagrams.net via its #R URL format and screenshotted at 2x.
//
// Usage: node export-diagrams.mjs <file.drawio> <output-prefix>
//   writes <output-prefix>-<page-slug>.png for each page
// Needs: playwright (npm i playwright; set NODE_PATH if installed elsewhere).
import fs from 'node:fs';
import zlib from 'node:zlib';
import { createRequire } from 'node:module';

const require = createRequire(import.meta.url);
const { chromium } = require('playwright');

const [, , input, prefix] = process.argv;
if (!input || !prefix) {
  console.error('usage: node export-diagrams.mjs <file.drawio> <output-prefix>');
  process.exit(2);
}

const xml = fs.readFileSync(input, 'utf8');
const pages = [...xml.matchAll(/<diagram\b[^>]*name="([^"]*)"[^>]*>([\s\S]*?)<\/diagram>/g)];
if (pages.length === 0) {
  console.error(`no uncompressed <diagram> pages found in ${input}`);
  process.exit(1);
}

// draw.io #R format: encodeURIComponent(xml) → raw deflate → base64 → URI-encoded.
const viewerURL = (pageXML) => {
  const data = zlib.deflateRawSync(Buffer.from(encodeURIComponent(`<mxfile>${pageXML}</mxfile>`)), { level: 9 });
  return 'https://viewer.diagrams.net/?lightbox=1&nav=1#R' + encodeURIComponent(data.toString('base64'));
};

const slug = (name) => name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');

const browser = await chromium.launch();
try {
  for (const [whole, name] of pages) {
    const w = Number(whole.match(/pageWidth="(\d+)"/)?.[1] ?? 1600) + 100;
    const h = Number(whole.match(/pageHeight="(\d+)"/)?.[1] ?? 1200) + 100;
    const page = await browser.newPage({ viewport: { width: w, height: h }, deviceScaleFactor: 2 });
    await page.goto(viewerURL(whole));
    await page.waitForSelector('foreignObject', { timeout: 30000 });
    await page.waitForTimeout(2500);
    const box = await page.evaluate(() => {
      const svg = [...document.querySelectorAll('svg')].find((s) => s.querySelector('foreignObject'));
      const r = svg.querySelector('g').getBoundingClientRect();
      return { x: r.x, y: r.y, width: r.width, height: r.height };
    });
    const pad = 16;
    const out = `${prefix}-${slug(name)}.png`;
    await page.screenshot({
      path: out,
      clip: { x: Math.max(0, box.x - pad), y: Math.max(0, box.y - pad), width: box.width + 2 * pad, height: box.height + 2 * pad },
    });
    console.log(`${out}  (${Math.round(box.width)} x ${Math.round(box.height)})`);
    await page.close();
  }
} finally {
  await browser.close();
}
