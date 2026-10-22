import { describe, expect, it } from 'vitest';
import { THEMES } from '../data/words';
import { pathCells } from './directions';
import { findOccurrences, generatePuzzle, GenerationError, letterAt } from './generate';
import { tierForIndex } from './levels';
import { reverse } from './normalize';
import type { Puzzle, Theme } from './types';

function gen(theme: Theme, index: number, blocklist?: string[]): Puzzle {
  for (const suffix of ['', ':1', ':2', ':3', ':4', ':5']) {
    try {
      return blocklist ? generatePuzzle(theme, index, suffix, blocklist) : generatePuzzle(theme, index, suffix);
    } catch (e) {
      if (!(e instanceof GenerationError)) throw e;
    }
  }
  throw new Error(`generation failed for ${theme.id}:${index}`);
}

describe('generatePuzzle', () => {
  for (const theme of THEMES) {
    it(`generates valid puzzles for ${theme.id}`, () => {
      const grids: string[] = [];
      for (let index = 1; index <= 12; index++) {
        const p = gen(theme, index);
        const spec = tierForIndex(index);
        expect(p.grid.length).toBe(p.rows * p.cols);
        for (const ch of p.grid) expect(ch).toMatch(/^[A-Z]$/);
        expect(p.placements.length).toBeGreaterThanOrEqual(spec.wordCount - 1);
        for (const pl of p.placements) {
          const letters = pathCells(pl, pl.dir, pl.word.length - 1).map((c) => letterAt(p, c)).join('');
          expect(letters).toBe(pl.word);
          expect(findOccurrences(p.grid, p.rows, p.cols, pl.word)).toBe(1);
          expect(spec.directions).toContain(pl.dir);
        }
        for (const a of p.placements) for (const b of p.placements) {
          if (a === b) continue;
          expect(a.word.includes(b.word) || a.word.includes(reverse(b.word))).toBe(false);
        }
        expect(gen(theme, index)).toEqual(p);
        grids.push(p.grid.join(''));
      }
      expect(new Set(grids).size).toBe(12);
    });
  }
  it('never places both CAFE and CAFEZINHO', () => {
    const t: Theme = { id: 'teste', name: 'Teste', emoji: 'x', words: ['Café', 'Cafezinho', 'Leite', 'Bolo', 'Suco', 'Ovos', 'Broa', 'Rosca'] };
    const words = gen(t, 1).placements.map((p) => p.word);
    expect(words.includes('CAFE') && words.includes('CAFEZINHO')).toBe(false);
  });
  it('avoids blocklisted words', () => {
    const frutas = THEMES.find((t) => t.id === 'frutas') as Theme;
    for (let index = 1; index <= 12; index++) {
      const p = gen(frutas, index, ['MANGA']);
      expect(findOccurrences(p.grid, p.rows, p.cols, 'MANGA')).toBe(0);
    }
  });
});
