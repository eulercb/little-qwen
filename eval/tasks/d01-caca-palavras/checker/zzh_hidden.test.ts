// Hidden acceptance tests (copied into src/core/ by the checker, then removed).
// Test names are checker item ids. Only behaviour stated in the brief is tested.
import { describe, expect, it } from 'vitest';
import * as dirs from './directions';
import * as gen from './generate';
import * as levels from './levels';
import * as norm from './normalize';
import * as prng from './prng';
import * as save from './save';
import * as sel from './select';
import { THEMES } from '../data/words';

type AnyFn = (...a: unknown[]) => unknown;
const fn = (m: Record<string, unknown>, k: string) => m[k] as AnyFn;
const count = (r: unknown) => (typeof r === 'number' ? r : Array.isArray(r) ? r.length : NaN);
interface P { word: string; row: number; col: number; dir: string }
interface Pz { key: string; rows: number; cols: number; grid: string[]; placements: P[]; tier: number }

function generate(theme: unknown, index: number, blocklist?: string[]): Pz {
  for (const s of ['', ':1', ':2', ':3', ':4', ':5']) {
    try {
      return (blocklist ? fn(gen, 'generatePuzzle')(theme, index, s, blocklist) : fn(gen, 'generatePuzzle')(theme, index, s)) as Pz;
    } catch (e) {
      if (!(e instanceof (gen as Record<string, unknown>).GenerationError as unknown as typeof Error)) throw e;
    }
  }
  throw new Error('generation failed');
}

describe('hidden', () => {
  it('u-normalize', () => {
    expect(norm.normalizeWord('Açúcar')).toBe('ACUCAR');
    expect(norm.normalizeWord('Pierrô')).toBe('PIERRO');
    expect(norm.isValidWord('Arraiá')).toBe(true);
    expect(norm.isValidWord('Bom-bom')).toBe(false);
    expect(norm.isValidWord('Bananeira')).toBe(true);
    expect(norm.isValidWord('Bananeiras')).toBe(false);
    expect(norm.reverse('ARARA')).toBe('ARARA');
  });
  it('u-prng-exact', () => {
    const r = prng.mulberry32(42);
    const got = [r(), r(), r(), r(), r()];
    expect(got).toEqual([0.6011037519201636, 0.44829055899754167, 0.8524657934904099, 0.6697340414393693, 0.17481389874592423]);
    expect(['praia:1', 'frutas:12', '', 'cafe:7:3'].map(prng.hashSeed)).toEqual([2211903053, 3992547669, 2166136261, 146551016]);
    expect(prng.rngInt(() => 0.999, 10)).toBe(9);
  });
  it('u-shuffle-exact', () => {
    const input = [0, 1, 2, 3, 4, 5, 6, 7, 8, 9];
    expect(prng.shuffle(input, prng.mulberry32(7))).toEqual([6, 5, 8, 1, 2, 3, 4, 7, 9, 0]);
    expect(input).toEqual([0, 1, 2, 3, 4, 5, 6, 7, 8, 9]);
  });
  it('u-directions', () => {
    expect(dirs.maxSteps({ row: 7, col: 7 }, 'NW', 8, 8)).toBe(7);
    expect(dirs.maxSteps({ row: 1, col: 6 }, 'NE', 8, 8)).toBe(1);
    expect(dirs.maxSteps({ row: 12, col: 0 }, 'S', 13, 10)).toBe(0);
    expect(dirs.pathCells({ row: 4, col: 4 }, 'W', 2)).toEqual([{ row: 4, col: 4 }, { row: 4, col: 3 }, { row: 4, col: 2 }]);
    expect(dirs.DIR_ORDER).toEqual(['E', 'SE', 'S', 'SW', 'W', 'NW', 'N', 'NE']);
    expect(dirs.angleOf('NE')).toBeCloseTo((7 * Math.PI) / 4);
  });
  it('u-levels', () => {
    expect(levels.PUZZLES_PER_THEME).toBe(12);
    expect(levels.TIERS.map((t) => [t.rows, t.cols, t.wordCount, t.minLen, t.maxLen])).toEqual([[8, 8, 5, 4, 7], [10, 9, 6, 4, 8], [11, 10, 7, 4, 9], [13, 10, 8, 4, 9]]);
    expect(levels.TIERS[2].directions).toEqual(['E', 'E', 'S', 'S', 'SE', 'NE', 'W', 'N']);
    expect(levels.tierForIndex(9).tier).toBe(3);
    expect(levels.puzzleKey('bichos', 11)).toBe('bichos:11');
  });
  it('u-select-cases', () => {
    const c = 40;
    const s = (r: number, cc: number, px: number, py: number, rows = 8, cols = 8) => sel.snapSelection({ row: r, col: cc }, px, py, c, rows, cols, null);
    expect(s(3, 3, 3.5 * c + 17, 3.5 * c, 8, 8).steps).toBe(0);
    expect(s(3, 3, 3.5 * c - 3 * c, 3.5 * c)).toEqual({ dir: 'W', steps: 3 });
    expect(s(3, 3, 3.5 * c, 3.5 * c - 2 * c)).toEqual({ dir: 'N', steps: 2 });
    expect(s(3, 3, 3.5 * c - 2 * c, 3.5 * c - 2 * c)).toEqual({ dir: 'NW', steps: 2 });
    expect(s(3, 3, 3.5 * c, 50 * c)).toEqual({ dir: 'S', steps: 4 });
    expect(s(0, 0, 0.5 * c, 0.5 * c + 3.1 * c, 13, 10)).toEqual({ dir: 'S', steps: 3 });
    expect(sel.selectionCells({ row: 2, col: 2 }, { dir: null, steps: 0 })).toEqual([{ row: 2, col: 2 }]);
    expect(sel.selectionCells({ row: 2, col: 2 }, { dir: 'SE', steps: 2 })).toEqual([{ row: 2, col: 2 }, { row: 3, col: 3 }, { row: 4, col: 4 }]);
  });
  it('u-select-hysteresis', () => {
    const c = 40;
    const at = (deg: number) => [3.5 * c + 3 * c * Math.cos((deg * Math.PI) / 180), 3.5 * c + 3 * c * Math.sin((deg * Math.PI) / 180)] as const;
    const s = (deg: number, prev: 'E' | 'S' | 'SE' | 'NE' | null) => sel.snapSelection({ row: 3, col: 3 }, ...at(deg), c, 8, 8, prev).dir;
    expect(s(30, 'E')).toBe('E');
    expect(s(35, 'E')).toBe('SE');
    expect(s(60, 'SE')).toBe('SE');
    expect(s(80, 'SE')).toBe('S');
    expect(s(-30, 'E')).toBe('E');
    expect(s(-25, null)).toBe('NE');
    expect(sel.snapSelection({ row: 3, col: 3 }, 3.5 * c + 5, 3.5 * c, c, 8, 8, 'SE').dir).toBe('SE');
  });
  it('u-matchword', () => {
    const rem = new Set(['ONDA', 'MARE']);
    expect(sel.matchWord('ADNO', rem)).toBe('ONDA');
    expect(sel.matchWord('MARE', rem)).toBe('MARE');
    expect(sel.matchWord('ERAMX', rem)).toBe(null);
  });
  it('u-save', () => {
    const d = save.defaultSave();
    expect(d).toEqual({ v: 1, solved: [], progress: null, settings: { haptics: true, theme: 'auto' } });
    const ok = { v: 1, solved: ['frutas:1'], progress: null, settings: { haptics: true, theme: 'light' } };
    expect(save.parseSave(JSON.stringify({ ...ok, junk: [1] }))).toEqual(ok);
    expect(save.parseSave(JSON.stringify({ ...ok, settings: { haptics: 'yes', theme: 'light' } }))).toEqual(d);
    expect(save.parseSave(JSON.stringify({ ...ok, solved: 'frutas:1' }))).toEqual(d);
    expect(save.parseSave(JSON.stringify({ ...ok, settings: { haptics: true, theme: 'sepia' } }))).toEqual(d);
    expect(save.parseSave('null')).toEqual(d);
    expect(JSON.parse(save.serializeSave(ok as never))).toEqual(ok);
  });
  it('u-generate-valid', () => {
    for (const t of THEMES) for (let i = 1; i <= 12; i++) {
      const p = generate(t, i);
      const spec = levels.tierForIndex(i);
      expect(p.key).toBe(`${t.id}:${i}`);
      expect([p.rows, p.cols, p.tier]).toEqual([spec.rows, spec.cols, spec.tier]);
      expect(p.grid.length).toBe(p.rows * p.cols);
      expect(p.grid.every((ch) => /^[A-Z]$/.test(ch) && !'KWY'.includes(ch) || p.placements.some((pl) => pl.word.includes(ch)))).toBe(true);
      expect(p.placements.length).toBeGreaterThanOrEqual(spec.wordCount - 1);
      expect(p.placements.length).toBeLessThanOrEqual(spec.wordCount);
      const themeWords = t.words.map(norm.normalizeWord);
      for (const pl of p.placements) {
        expect(themeWords).toContain(pl.word);
        expect(spec.directions).toContain(pl.dir);
        const cells = dirs.pathCells({ row: pl.row, col: pl.col }, pl.dir as 'E', pl.word.length - 1);
        expect(cells.map((cc) => p.grid[cc.row * p.cols + cc.col]).join('')).toBe(pl.word);
        expect(count(fn(gen, 'findOccurrences')(p.grid, p.rows, p.cols, pl.word))).toBe(1);
      }
    }
  });
  it('u-generate-deterministic', () => {
    const t = THEMES.find((x) => x.id === 'praia');
    expect(generate(t, 4)).toEqual(generate(t, 4));
    expect(generate(t, 4).grid).not.toEqual(generate(t, 5).grid);
    expect(fn(gen, 'generatePuzzle')(t, 4, ':2')).not.toEqual(fn(gen, 'generatePuzzle')(t, 4, ''));
  });
  it('u-generate-substring', () => {
    const t = { id: 'zzh', name: 'Z', emoji: 'z', words: ['Folia', 'Folião', 'Samba', 'Bloco', 'Frevo', 'Apito', 'Surdo', 'Trio', 'Cuíca'] };
    for (let i = 1; i <= 12; i++) {
      const w = generate(t, i).placements.map((p) => p.word);
      expect(w.includes('FOLIA') && w.includes('FOLIAO')).toBe(false);
    }
  });
  it('u-generate-blocklist', () => {
    const t = THEMES.find((x) => x.id === 'bichos');
    for (let i = 1; i <= 12; i++) {
      const p = generate(t, i, ['Gato', 'ARARA']);
      expect(p.placements.map((x) => x.word)).not.toContain('GATO');
      expect(count(fn(gen, 'findOccurrences')(p.grid, p.rows, p.cols, 'GATO'))).toBe(0);
      expect(count(fn(gen, 'findOccurrences')(p.grid, p.rows, p.cols, 'ARARA'))).toBe(0);
    }
  });
  it('u-letterat', () => {
    const p = generate(THEMES[0], 7);
    expect(fn(gen, 'letterAt')(p, { row: 2, col: 3 })).toBe(p.grid[2 * p.cols + 3]);
  });
});
