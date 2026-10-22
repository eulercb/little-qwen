// Browser acceptance tests. Usage: node e2e.cjs <playwright-core dir> <chrome path> <base url>
// Prints PASS|FAIL <item> lines. Uses only behaviour and selectors stated in the brief.
const { chromium } = require(process.argv[2]);
const CHROME = process.argv[3];
const BASE = process.argv[4];
const out = (ok, item, detail = '') => console.log(`${ok ? 'PASS' : 'FAIL'} ${item} ${detail}`.trim());
const DIRS = [[0, 1], [1, 1], [1, 0], [1, -1], [0, -1], [-1, -1], [-1, 0], [-1, 1]];

async function gridInfo(page) {
  return page.$$eval('.cell', (els) => els.map((e) => {
    const r = e.getBoundingClientRect();
    return { r: Number(e.dataset.r), c: Number(e.dataset.c), ch: e.textContent.trim(), x: r.x + r.width / 2, y: r.y + r.height / 2, w: r.width };
  }));
}
async function chipWords(page) {
  return page.$$eval('.chip', (els) => els.map((e) => ({ word: e.dataset.word || '', disabled: e.disabled })));
}
function locate(cells, word) {
  const at = new Map(cells.map((c) => [`${c.r},${c.c}`, c]));
  for (const s of cells) for (const [dr, dc] of DIRS) {
    const path = [];
    for (let i = 0; i < word.length; i++) {
      const c = at.get(`${s.r + dr * i},${s.c + dc * i}`);
      if (!c || c.ch !== word[i]) break;
      path.push(c);
    }
    if (path.length === word.length) return path;
  }
  return null;
}
async function drag(page, from, to) {
  await page.mouse.move(from.x, from.y);
  await page.mouse.down();
  for (let i = 1; i <= 10; i++) await page.mouse.move(from.x + ((to.x - from.x) * i) / 10, from.y + ((to.y - from.y) * i) / 10);
  await page.mouse.up();
  await page.waitForTimeout(250);
}
async function noScroll(page) {
  return page.evaluate(() => {
    const se = document.scrollingElement || document.documentElement;
    return se.scrollHeight <= window.innerHeight + 1 && se.scrollWidth <= window.innerWidth + 1;
  });
}
async function step(item, fn) {
  try { const r = await fn(); out(r === true, item, r === true ? '' : String(r)); } catch (e) { out(false, item, String(e).slice(0, 200)); }
}

(async () => {
  const browser = await chromium.launch({ executablePath: CHROME, args: ['--no-sandbox'] });
  const ctx = await browser.newContext({ viewport: { width: 360, height: 740 }, hasTouch: false, locale: 'pt-BR' });
  const page = await ctx.newPage();
  page.setDefaultTimeout(8000);
  const go = async (hash) => { await page.goto(`${BASE}/${hash}`); await page.waitForTimeout(400); };

  await step('e-home', async () => {
    await go('#/');
    const h1 = (await page.textContent('h1') || '').trim();
    const rows = await page.$$eval('.theme-row:not(.continue)', (e) => e.map((x) => x.textContent));
    if (h1 !== 'Caça-Palavras') return `h1=${h1}`;
    if (rows.length !== 8) return `rows=${rows.length}`;
    return rows.every((t) => t.includes('0 de 12')) || 'missing 0 de 12';
  });
  await step('e-puzzle-tier1', async () => {
    await go('#/p/praia/1');
    const cells = await gridInfo(page);
    const chips = await chipWords(page);
    if (cells.length !== 64) return `cells=${cells.length}`;
    if (!cells.every((c) => /^[A-Z]$/.test(c.ch))) return 'non-letter cell';
    if (chips.length < 4 || chips.length > 5) return `chips=${chips.length}`;
    if (Math.min(...cells.map((c) => c.w)) < 28) return 'cell < 28px';
    return (await noScroll(page)) || 'page scrolls';
  });
  await step('e-trace-hint', async () => ((await page.textContent('.trace')) || '').trim() === 'Arraste o dedo sobre as letras' || 'trace text');
  await step('e-puzzle-tier4', async () => {
    await go('#/p/praia/10');
    const cells = await gridInfo(page);
    if (cells.length !== 130) return `cells=${cells.length}`;
    if (Math.min(...cells.map((c) => c.w)) < 28) return 'cell < 28px';
    const wordsBottom = await page.$eval('.words', (e) => e.getBoundingClientRect().bottom);
    if (wordsBottom > 741) return `chips below fold (${wordsBottom})`;
    return (await noScroll(page)) || 'page scrolls';
  });

  let words = [];
  await step('e-drag-forward', async () => {
    await go('#/p/praia/1');
    const cells = await gridInfo(page);
    words = (await chipWords(page)).map((c) => c.word);
    const path = locate(cells, words[0]);
    if (!path) return `cannot locate ${words[0]} via data-word`;
    await drag(page, path[0], path[path.length - 1]);
    const chip = await page.$(`.chip[data-word="${words[0]}"]`);
    const dis = await chip.evaluate((e) => e.disabled && getComputedStyle(e).textDecorationLine.includes('line-through'));
    const pills = await page.$$eval('svg g.found line', (l) => l.length);
    return (dis && pills === 1) || `disabled+strike=${dis} pills=${pills}`;
  });
  await step('e-drag-reverse', async () => {
    const cells = await gridInfo(page);
    const path = locate(cells, words[1]);
    await drag(page, path[path.length - 1], path[0]);
    return (await page.$eval(`.chip[data-word="${words[1]}"]`, (e) => e.disabled)) || 'not found reversed';
  });
  await step('e-miss', async () => {
    const cells = await gridInfo(page);
    const rem = new Set(words.slice(2));
    let pair = null;
    for (const a of cells) {
      const b = cells.find((x) => x.r === a.r && x.c === a.c + 1);
      if (b && !rem.has(a.ch + b.ch) && !rem.has(b.ch + a.ch)) { pair = [a, b]; break; }
    }
    const before = (await chipWords(page)).filter((c) => c.disabled).length;
    await drag(page, pair[0], pair[1]);
    await page.waitForTimeout(400);
    const after = (await chipWords(page)).filter((c) => c.disabled).length;
    const hidden = await page.$eval('svg line.active', (l) => l.style.display === 'none' || getComputedStyle(l).opacity === '0');
    const pills = await page.$$eval('svg g.found line', (l) => l.length);
    return (before === after && hidden && pills === 2) || `before=${before} after=${after} hidden=${hidden} pills=${pills}`;
  });
  await step('e-hint', async () => {
    await page.click(`.chip[data-word="${words[2]}"]`);
    await page.waitForTimeout(150);
    return (await page.$$eval('.cell.hint', (e) => e.length)) === 1 || 'no .cell.hint';
  });
  await step('e-persist', async () => {
    await page.waitForTimeout(1000);
    await page.reload();
    await page.waitForTimeout(500);
    const dis = (await chipWords(page)).filter((c) => c.disabled).length;
    const pills = await page.$$eval('svg g.found line', (l) => l.length);
    await go('#/');
    const cont = await page.$$eval('.theme-row.continue', (e) => e.map((x) => x.textContent));
    return (dis === 2 && pills === 2 && cont.length === 1 && cont[0].includes('Praia, fase 1')) || `dis=${dis} pills=${pills} continue=${JSON.stringify(cont)}`;
  });
  await step('e-complete', async () => {
    await go('#/p/praia/1');
    const cells = await gridInfo(page);
    for (const c of await chipWords(page)) {
      if (c.disabled) continue;
      const path = locate(cells, c.word);
      await drag(page, path[0], path[path.length - 1]);
    }
    await page.waitForTimeout(800);
    const card = await page.$('.done-card');
    if (!card) return 'no .done-card';
    const txt = await card.textContent();
    if (!txt.includes('Todas encontradas!')) return 'missing allFound';
    await page.click('.done-card >> text=Próxima fase');
    await page.waitForTimeout(400);
    const hash = await page.evaluate(() => location.hash);
    await go('#/');
    const row = await page.$eval('.theme-row[data-theme="praia"]', (e) => e.textContent);
    return (hash === '#/p/praia/2' && row.includes('1 de 12')) || `hash=${hash} row=${row}`;
  });
  await step('e-settings-dark', async () => {
    await go('#/');
    await page.click('[aria-label="Ajustes"]');
    await page.waitForTimeout(200);
    const body = await page.textContent('body');
    if (!body.includes('Vibração') || !body.includes('Aparência')) return 'sheet rows missing';
    await page.click('text=Escuro');
    await page.waitForTimeout(200);
    const t1 = await page.evaluate(() => document.documentElement.dataset.theme);
    await page.reload();
    await page.waitForTimeout(400);
    const t2 = await page.evaluate(() => document.documentElement.dataset.theme);
    return (t1 === 'dark' && t2 === 'dark') || `theme ${t1}/${t2}`;
  });
  await step('e-offline', async () => {
    await go('#/p/frutas/1');
    await page.evaluate(() => navigator.serviceWorker && navigator.serviceWorker.ready);
    await page.waitForTimeout(1500);
    await ctx.setOffline(true);
    await page.reload();
    await page.waitForTimeout(800);
    const n = (await gridInfo(page)).length;
    await ctx.setOffline(false);
    return n === 64 || `cells offline=${n}`;
  });
  await browser.close();
})().catch((e) => { console.log(`FAIL e2e-crash ${String(e).slice(0, 200)}`); process.exit(0); });
