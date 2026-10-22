import { describe, expect, it } from 'vitest';
import { isValidWord, normalizeWord } from '../core/normalize';
import { LETTER_WEIGHTS } from './letters';
import { THEMES } from './words';

describe('data', () => {
  it('has unique theme ids', () => { expect(new Set(THEMES.map((t) => t.id)).size).toBe(THEMES.length); });
  it('has valid, unique words per theme', () => {
    for (const t of THEMES) {
      expect(t.words.length).toBeGreaterThanOrEqual(20);
      for (const w of t.words) expect(isValidWord(w)).toBe(true);
      const norm = t.words.map(normalizeWord);
      expect(new Set(norm).size).toBe(norm.length);
      expect(norm.filter((w) => w.length <= 7).length).toBeGreaterThanOrEqual(8);
    }
  });
  it('has sane letter weights', () => {
    for (const [k, v] of Object.entries(LETTER_WEIGHTS)) {
      expect(k).toMatch(/^[A-Z]$/);
      expect(['K', 'W', 'Y']).not.toContain(k);
      expect(v).toBeGreaterThan(0);
    }
  });
});
