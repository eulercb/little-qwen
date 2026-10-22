import { describe, expect, it } from 'vitest';
import { TIERS, tierForIndex } from './levels';

describe('levels', () => {
  it('maps indices to tiers', () => {
    const tiers = Array.from({ length: 12 }, (_, i) => tierForIndex(i + 1).tier);
    expect(tiers).toEqual([1, 1, 1, 2, 2, 2, 3, 3, 3, 4, 4, 4]);
  });
  it('keeps columns <= 10', () => { for (const t of TIERS) expect(t.cols).toBeLessThanOrEqual(10); });
});
