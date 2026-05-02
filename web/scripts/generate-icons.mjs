// Generate the PWA icon set by rasterizing an inline SVG with Playwright.
// Run via `node scripts/generate-icons.mjs` from web/.
//
// Output paths (overwritten):
//   static/icon-192.png         — non-maskable
//   static/icon-512.png         — non-maskable
//   static/icon-512-maskable.png — extra-padded for adaptive Android crops
//   static/apple-touch-icon.png — 180x180, iOS A2HS
//
// The icon is a hot-pink microphone on the SoundStage dark surface, matching
// the design tokens in src/lib/styles/base.css.

import { chromium } from "playwright";
import { writeFile } from "fs/promises";
import { join, dirname } from "path";
import { fileURLToPath } from "url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const STATIC_DIR = join(__dirname, "..", "static");

// Inner content stays inside a 0..512 viewBox; the wrapper draws the
// background. For maskable, we shrink the inner content into the W3C "safe
// zone" (inner 80% of the canvas) and let the background bleed to the edge.
function svg({ size, maskable }) {
  const inset = maskable ? 51 : 0; // 10% inset on each side
  const inner = `
    <!-- Microphone capsule (rounded vertical pill) -->
    <rect x="216" y="96" width="80" height="170" rx="40" fill="#ff2d7b"/>
    <!-- Yoke: U-shape under the capsule -->
    <path d="M 156 220 V 250 A 100 100 0 0 0 356 250 V 220"
          stroke="#ff2d7b" stroke-width="14" fill="none" stroke-linecap="round"/>
    <!-- Stem from yoke down -->
    <line x1="256" y1="350" x2="256" y2="400" stroke="#ff2d7b" stroke-width="14" stroke-linecap="round"/>
    <!-- Base bar -->
    <rect x="196" y="394" width="120" height="14" rx="7" fill="#ff2d7b"/>
  `;
  return `<svg xmlns="http://www.w3.org/2000/svg" width="${size}" height="${size}" viewBox="0 0 512 512">
    <rect width="512" height="512" fill="#0a0a0f"/>
    <g transform="translate(${inset} ${inset}) scale(${(512 - inset * 2) / 512})">
      ${inner}
    </g>
  </svg>`;
}

async function rasterize(browser, sizes) {
  const context = await browser.newContext();
  const page = await context.newPage();
  for (const { file, size, maskable = false } of sizes) {
    const html = `<!doctype html><html><body style="margin:0;padding:0;background:#0a0a0f">${svg({ size, maskable })}</body></html>`;
    await page.setViewportSize({ width: size, height: size });
    await page.setContent(html);
    const buf = await page.locator("svg").screenshot({ omitBackground: false });
    await writeFile(join(STATIC_DIR, file), buf);
    console.log(`wrote static/${file} (${size}x${size}${maskable ? " maskable" : ""})`);
  }
  await context.close();
}

// Prefer the standard Playwright env var so non-NixOS contributors can
// regenerate icons without editing this file; fall back to the system
// chromium path that ships in the dev shell.
const browser = await chromium.launch({
  executablePath:
    process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH ?? "/run/current-system/sw/bin/chromium",
});
try {
  await rasterize(browser, [
    { file: "icon-192.png", size: 192 },
    { file: "icon-512.png", size: 512 },
    { file: "icon-512-maskable.png", size: 512, maskable: true },
    { file: "apple-touch-icon.png", size: 180 },
  ]);
} finally {
  await browser.close();
}
