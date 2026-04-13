/**
 * Screenshot utility for capturing Storybook components
 *
 * Usage:
 *   npx tsx scripts/screenshot.ts <story-id>              # single story
 *   npx tsx scripts/screenshot.ts <id1> <id2> ...         # multiple stories
 *   npx tsx scripts/screenshot.ts --grep <pattern>        # filter by substring/regex
 *   npx tsx scripts/screenshot.ts --all                   # every story
 *
 * Options:
 *   --port <number>       Storybook port (default: 6006)
 *   --viewport <preset>   Viewport preset: phone (default), tablet, desktop
 *   --suffix <string>     Append to screenshot filename (e.g., -dark)
 *
 * Examples:
 *   npx tsx scripts/screenshot.ts design-system--app-mockup
 *   npx tsx scripts/screenshot.ts --grep "design-system"
 *   npx tsx scripts/screenshot.ts --all
 *   npx tsx scripts/screenshot.ts --all --viewport desktop
 *
 * Story paths follow the pattern: category-component--story-name (lowercase, hyphens)
 *
 * Output: screenshots/<story-path>.png
 */

import { execFileSync } from "child_process";
import { mkdir } from "fs/promises";
import { dirname, join } from "path";
import { fileURLToPath } from "url";

import { chromium } from "playwright";

const __dirname = dirname(fileURLToPath(import.meta.url));
const screenshotsDir = join(__dirname, "..", "screenshots");

const VIEWPORTS = {
  phone: { width: 390, height: 844 },    // iPhone 14 / typical modern phone
  tablet: { width: 768, height: 1024 },   // iPad
  desktop: { width: 1200, height: 900 },  // Desktop
} as const;

function findChromium(): string | undefined {
  if (process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH) {
    return process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH;
  }

  const candidates = [
    "chromium",
    "chromium-browser",
    "google-chrome-stable",
    "google-chrome",
  ];
  for (const cmd of candidates) {
    try {
      const path = execFileSync("which", [cmd], { encoding: "utf-8" }).trim();
      if (path) return path;
    } catch {
      // Command not found, try next
    }
  }

  return undefined;
}

function parseArgs(): { mode: "all" | "grep" | "ids"; port: number; viewport: keyof typeof VIEWPORTS; suffix?: string; pattern?: string; ids?: string[] } {
  const args = process.argv.slice(2);
  let port = 6006;
  let viewport: keyof typeof VIEWPORTS = "phone";
  let suffix: string | undefined;
  const ids: string[] = [];
  let mode: "all" | "grep" | "ids" = "ids";
  let pattern: string | undefined;

  for (let index = 0; index < args.length; index++) {
    const arg = args[index];
    if (arg === "--port") {
      port = Number(args[++index]);
    } else if (arg === "--viewport") {
      viewport = args[++index] as keyof typeof VIEWPORTS;
    } else if (arg === "--suffix") {
      suffix = args[++index];
    } else if (arg === "--all") {
      mode = "all";
    } else if (arg === "--grep") {
      mode = "grep";
      pattern = args[++index];
    } else if (!arg.startsWith("-")) {
      ids.push(arg);
    }
  }

  if (mode === "ids" && ids.length === 0) {
    console.log("Usage:");
    console.log("  npx tsx scripts/screenshot.ts <story-id>          # single story");
    console.log("  npx tsx scripts/screenshot.ts <id1> <id2> ...     # multiple stories");
    console.log("  npx tsx scripts/screenshot.ts --grep <pattern>    # filter by substring/regex");
    console.log("  npx tsx scripts/screenshot.ts --all               # every story");
    console.log("  --port <number>       Storybook port (default: 6006)");
    console.log("  --viewport <preset>   phone (default), tablet, desktop");
    console.log("  --suffix <string>     Append to filename (e.g., -dark)");
    process.exit(1);
  }

  return { mode, port, viewport, suffix, pattern, ids: ids.length > 0 ? ids : undefined };
}

async function getAllStoryIds(port: number): Promise<string[]> {
  const response = await fetch(`http://localhost:${String(port)}/index.json`);
  const index = (await response.json()) as {
    entries: Record<string, { id: string; type: string }>;
  };
  return Object.values(index.entries)
    .filter((entry) => entry.type === "story")
    .map((entry) => entry.id);
}

async function captureStories(storyIds: string[], port: number, viewport: keyof typeof VIEWPORTS, suffix?: string): Promise<void> {
  await mkdir(screenshotsDir, { recursive: true });

  const executablePath = findChromium();
  const vp = VIEWPORTS[viewport];
  const browser = await chromium.launch({
    executablePath,
    channel: executablePath ? undefined : "chromium",
  });
  const page = await browser.newPage({ viewport: vp });

  const filenameSuffix = suffix ?? "";

  console.log(`Capturing ${String(storyIds.length)} ${storyIds.length === 1 ? "story" : "stories"} @ ${viewport} (${String(vp.width)}x${String(vp.height)})\n`);

  for (const storyId of storyIds) {
    const url = `http://localhost:${String(port)}/iframe.html?id=${storyId}&viewMode=story`;
    console.log(`  ${storyId}${filenameSuffix}`);

    try {
      await page.goto(url, { waitUntil: "networkidle" });
      await page.waitForTimeout(500);

      const outputPath = join(screenshotsDir, `${storyId}${filenameSuffix}.png`);
      await page.screenshot({ path: outputPath, fullPage: true });
    } catch (error) {
      console.error(`  Failed: ${String(error)}`);
    }
  }

  await browser.close();
  console.log(`\nDone! ${String(storyIds.length)} screenshots in ${screenshotsDir}`);
}

async function main(): Promise<void> {
  const { mode, port, viewport, suffix, pattern, ids } = parseArgs();

  let storyIds: string[];

  if (mode === "all") {
    storyIds = await getAllStoryIds(port);
  } else if (mode === "grep") {
    const allIds = await getAllStoryIds(port);
    const regex = new RegExp(pattern!, "i");
    storyIds = allIds.filter((id) => regex.test(id));
    if (storyIds.length === 0) {
      console.error(`No stories match pattern: ${pattern!}`);
      process.exit(1);
    }
  } else {
    storyIds = ids!;
  }

  await captureStories(storyIds, port, viewport, suffix);
}

void main();
