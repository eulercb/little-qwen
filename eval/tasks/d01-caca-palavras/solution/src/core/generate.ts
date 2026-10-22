import { BLOCKLIST } from '../data/blocklist';
import { LETTER_WEIGHTS } from '../data/letters';
import { DIR_ORDER, maxSteps, pathCells } from './directions';
import { puzzleKey, tierForIndex } from './levels';
import { normalizeWord, reverse } from './normalize';
import { hashSeed, mulberry32, shuffle, type Rng } from './prng';
import type { Cell, Dir, Placement, Puzzle, Theme, TierSpec } from './types';

export class GenerationError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'GenerationError';
  }
}

interface Candidate { display: string; word: string }

const FILL_RETRIES = 20;
const PLACEMENT_RETRIES = 10;

export function letterAt(puzzle: Puzzle, cell: Cell): string {
  return puzzle.grid[cell.row * puzzle.cols + cell.col];
}

/** Number of distinct cell sets where `word` reads in any of the 8 directions. */
export function findOccurrences(grid: string[], rows: number, cols: number, word: string): number {
  const seen = new Set<string>();
  const steps = word.length - 1;
  for (let row = 0; row < rows; row++) {
    for (let col = 0; col < cols; col++) {
      if (grid[row * cols + col] !== word[0]) continue;
      for (const d of DIR_ORDER) {
        if (maxSteps({ row, col }, d, rows, cols) < steps) continue;
        const cells = pathCells({ row, col }, d, steps);
        if (cells.every((c, i) => grid[c.row * cols + c.col] === word[i])) {
          seen.add(cells.map((c) => `${c.row},${c.col}`).sort().join('|'));
        }
      }
    }
  }
  return seen.size;
}

function conflicts(v: string, w: string): boolean {
  return v.includes(w) || w.includes(v) || v.includes(reverse(w)) || w.includes(reverse(v));
}

function buildFiller(rng: Rng): () => string {
  const letters = Object.keys(LETTER_WEIGHTS);
  const cumulative: number[] = [];
  let total = 0;
  for (const l of letters) {
    total += LETTER_WEIGHTS[l];
    cumulative.push(total);
  }
  return () => {
    const x = rng() * total;
    const i = cumulative.findIndex((c) => x < c);
    return letters[i === -1 ? letters.length - 1 : i];
  };
}

function tryPlace(grid: string[], spec: TierSpec, c: Candidate, rng: Rng): Placement | null {
  const steps = c.word.length - 1;
  const options: { row: number; col: number; dir: Dir }[] = [];
  for (let row = 0; row < spec.rows; row++) {
    for (let col = 0; col < spec.cols; col++) {
      for (const dir of spec.directions) {
        if (maxSteps({ row, col }, dir, spec.rows, spec.cols) >= steps) options.push({ row, col, dir });
      }
    }
  }
  for (const o of shuffle(options, rng)) {
    const cells = pathCells(o, o.dir, steps);
    if (cells.every((cell, i) => {
      const cur = grid[cell.row * spec.cols + cell.col];
      return cur === '' || cur === c.word[i];
    })) {
      cells.forEach((cell, i) => { grid[cell.row * spec.cols + cell.col] = c.word[i]; });
      return { word: c.word, display: c.display, row: o.row, col: o.col, dir: o.dir };
    }
  }
  return null;
}

function placeAll(spec: TierSpec, candidates: Candidate[], rng: Rng): { grid: string[]; placements: Placement[] } {
  const collected: Candidate[] = [];
  const reserve: Candidate[] = [];
  for (const c of candidates) {
    if (collected.length < spec.wordCount && !collected.some((v) => conflicts(v.word, c.word))) collected.push(c);
    else reserve.push(c);
  }
  const pending = [...collected].sort((a, b) => b.word.length - a.word.length); // stable
  const grid: string[] = new Array<string>(spec.rows * spec.cols).fill('');
  const placements: Placement[] = [];
  while (pending.length > 0 && placements.length < spec.wordCount) {
    const next = pending.shift() as Candidate;
    const p = tryPlace(grid, spec, next, rng);
    if (p) {
      placements.push(p);
      continue;
    }
    // Replace the dropped word with the next compatible reserve word.
    while (reserve.length > 0) {
      const r = reserve.shift() as Candidate;
      const others = [...placements.map((x) => x.word), ...pending.map((x) => x.word)];
      if (others.some((w) => conflicts(w, r.word))) continue;
      pending.unshift(r);
      break;
    }
  }
  return { grid, placements };
}

export function generatePuzzle(theme: Theme, index: number, seedSuffix = '', blocklist: string[] = BLOCKLIST): Puzzle {
  const key = puzzleKey(theme.id, index);
  const rng = mulberry32(hashSeed(key + seedSuffix));
  const spec = tierForIndex(index);
  const maxLen = Math.min(spec.maxLen, spec.cols, spec.rows);
  const blocked = blocklist.map(normalizeWord);
  const candidates = shuffle(
    theme.words
      .map((display) => ({ display, word: normalizeWord(display) }))
      .filter((c) => c.word.length >= spec.minLen && c.word.length <= maxLen && !blocked.includes(c.word)),
    rng,
  );
  const fill = buildFiller(rng);
  for (let attempt = 0; attempt <= PLACEMENT_RETRIES; attempt++) {
    const { grid, placements } = placeAll(spec, candidates, rng);
    if (placements.length < spec.wordCount - 1) continue;
    for (let f = 0; f <= FILL_RETRIES; f++) {
      const filled = grid.map((ch) => ch || fill());
      const ok = placements.every((p) => findOccurrences(filled, spec.rows, spec.cols, p.word) === 1)
        && blocked.every((b) => b.length === 0 || findOccurrences(filled, spec.rows, spec.cols, b) === 0);
      if (ok) {
        return { key, themeId: theme.id, index, tier: spec.tier, rows: spec.rows, cols: spec.cols, grid: filled, placements };
      }
    }
  }
  throw new GenerationError(`could not generate ${key}${seedSuffix}`);
}
