import { describe, expect, it } from 'vitest';
import { hashSeed, mulberry32, shuffle } from './prng';

const take = (seed: number, n: number) => { const r = mulberry32(seed); return Array.from({ length: n }, () => r()); };

describe('prng', () => {
  it('is deterministic per seed', () => { expect(take(7, 20)).toEqual(take(7, 20)); });
  it('differs across seeds', () => { expect(take(7, 20)).not.toEqual(take(8, 20)); });
  it('stays in [0, 1)', () => { for (const v of take(123, 1000)) { expect(v).toBeGreaterThanOrEqual(0); expect(v).toBeLessThan(1); } });
  it('shuffles a copy deterministically', () => {
    const input = [1, 2, 3, 4, 5, 6, 7, 8];
    const a = shuffle(input, mulberry32(1));
    expect([...a].sort()).toEqual(input);
    expect(input).toEqual([1, 2, 3, 4, 5, 6, 7, 8]);
    expect(shuffle(input, mulberry32(1))).toEqual(a);
  });
  it('hashes praia:1', () => { expect(hashSeed('praia:1')).toBe(2211903053); });
});
