import { pathCells } from '../core/directions';
import { GenerationError, generatePuzzle } from '../core/generate';
import { PUZZLES_PER_THEME } from '../core/levels';
import type { Cell, FoundWord, Puzzle, Theme } from '../core/types';
import { themeById } from '../data/words';
import { renderGrid } from './grid';
import { attachPointer } from './pointer';
import { navigate } from './router';
import { getSave, saveState } from './storage';
import { STR } from './strings';

function generate(theme: Theme, index: number): Puzzle {
  let last: unknown;
  for (const suffix of ['', ':1', ':2', ':3', ':4', ':5']) {
    try {
      return generatePuzzle(theme, index, suffix);
    } catch (e) {
      if (!(e instanceof GenerationError)) throw e;
      last = e;
    }
  }
  throw last;
}

function el<K extends keyof HTMLElementTagNameMap>(tag: K, cls?: string, text?: string): HTMLElementTagNameMap[K] {
  const e = document.createElement(tag);
  if (cls) e.className = cls;
  if (text !== undefined) e.textContent = text;
  return e;
}

function vibrate(pattern: number | number[]): void {
  if (getSave().settings.haptics) navigator.vibrate?.(pattern);
}

export function renderPuzzle(app: HTMLElement, themeId: string, index: number): void {
  const theme = themeById(themeId) as Theme;
  const puzzle = generate(theme, index);
  const save = getSave();
  const found: FoundWord[] = [];
  const remaining = new Set(puzzle.placements.map((p) => p.word));
  if (save.progress?.key === puzzle.key) {
    for (const w of save.progress.found) {
      const p = puzzle.placements.find((x) => x.word === w);
      if (!p || !remaining.has(w)) continue;
      found.push({ word: w, cells: pathCells(p, p.dir, p.word.length - 1), colorIndex: found.length % 8 });
      remaining.delete(w);
    }
  }

  const head = el('header', 'puzzle-head');
  const back = el('button', 'icon-btn back', '‹');
  back.setAttribute('aria-label', STR.back);
  back.addEventListener('click', () => navigate('#/'));
  const title = el('div', 'title');
  title.append(el('strong', undefined, theme.name), el('span', undefined, STR.phase(index, PUZZLES_PER_THEME)));
  head.append(back, title);
  const trace = el('div', 'trace');
  trace.setAttribute('aria-live', 'polite');
  const area = el('div', 'grid-area');
  const words = el('div', 'words');
  app.replaceChildren(head, trace, area, words);

  const idleTrace = () => {
    trace.style.color = '';
    trace.textContent = found.length === 0 ? STR.dragHint : '';
  };
  idleTrace();
  const view = renderGrid(area, puzzle, () => found);

  const chips = new Map<string, HTMLButtonElement>();
  const sorted = [...puzzle.placements].sort((a, b) => a.display.localeCompare(b.display, 'pt-BR'));
  for (const p of sorted) {
    const chip = el('button', 'chip', p.display);
    chip.dataset.word = p.word;
    chips.set(p.word, chip);
    words.append(chip);
  }
  const markChip = (f: FoundWord) => {
    const chip = chips.get(f.word);
    if (!chip) return;
    chip.classList.add('found');
    chip.style.background = `var(--p${f.colorIndex})`;
    chip.disabled = true;
  };
  found.forEach(markChip);

  let hinting = false;
  words.addEventListener('click', (e) => {
    const chip = (e.target as HTMLElement).closest('.chip') as HTMLButtonElement | null;
    if (!chip || chip.disabled || hinting) return;
    const p = puzzle.placements.find((x) => x.word === chip.dataset.word);
    if (!p) return;
    const cellEl = view.cellEl(p);
    hinting = true;
    cellEl.classList.add('hint');
    window.setTimeout(() => {
      cellEl.classList.remove('hint');
      hinting = false;
    }, 900);
  });

  let traceTimer = 0;
  const onFound = (word: string, cells: Cell[]) => {
    const f: FoundWord = { word, cells, colorIndex: found.length % 8 };
    found.push(f);
    remaining.delete(word);
    view.addFound(f);
    markChip(f);
    vibrate(15);
    window.clearTimeout(traceTimer);
    trace.textContent = word;
    trace.style.color = `var(--p${f.colorIndex})`;
    traceTimer = window.setTimeout(idleTrace, 600);
    if (remaining.size > 0) {
      save.progress = { key: puzzle.key, found: found.map((x) => x.word) };
      saveState(save);
      return;
    }
    if (!save.solved.includes(puzzle.key)) save.solved.push(puzzle.key);
    save.progress = null;
    saveState(save);
    vibrate([15, 60, 25]);
    window.setTimeout(() => showDone(theme, index), 350);
  };
  attachPointer(view, puzzle, {
    remaining: () => remaining,
    onFound,
    onTrace: (letters) => {
      window.clearTimeout(traceTimer);
      if (letters === null) idleTrace();
      else {
        trace.style.color = '';
        trace.textContent = letters;
      }
    },
  });
}

function showDone(theme: Theme, index: number): void {
  const backdrop = el('div', 'done-backdrop');
  const card = el('div', 'done-card');
  card.append(el('h2', undefined, STR.allFound), el('p', undefined, `${theme.name}, fase ${index} de ${PUZZLES_PER_THEME}`));
  const close = () => backdrop.remove();
  if (index < PUZZLES_PER_THEME) {
    const next = el('button', 'btn primary', STR.nextPuzzle);
    next.addEventListener('click', () => { close(); navigate(`#/p/${theme.id}/${index + 1}`); });
    card.append(next);
  } else {
    card.append(el('p', undefined, STR.themeComplete(theme.name)));
  }
  const choose = el('button', 'btn secondary', STR.chooseTheme);
  choose.addEventListener('click', () => { close(); navigate('#/'); });
  card.append(choose);
  backdrop.append(card);
  document.body.append(backdrop);
  window.addEventListener('hashchange', close, { once: true });
}
