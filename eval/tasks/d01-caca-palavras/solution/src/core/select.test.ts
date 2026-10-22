import { describe, expect, it } from 'vitest';
import { matchWord, snapSelection } from './select';

const c = 40;
const A = { row: 3, col: 3 };
const at = (dx: number, dy: number) => [(A.col + 0.5) * c + dx, (A.row + 0.5) * c + dy] as const;
const deg = (d: number, r = 3 * c) => at(r * Math.cos((d * Math.PI) / 180), r * Math.sin((d * Math.PI) / 180));

describe('snapSelection', () => {
  it('has a dead zone', () => { expect(snapSelection(A, ...at(10, 0), c, 8, 8, null).steps).toBe(0); });
  it('snaps to the 8 directions', () => {
    expect(snapSelection(A, ...at(3 * c, 0), c, 8, 8, null)).toEqual({ dir: 'E', steps: 3 });
    expect(snapSelection(A, ...at(0, 3 * c), c, 8, 8, null)).toEqual({ dir: 'S', steps: 3 });
    expect(snapSelection(A, ...at(3 * c, -3 * c), c, 8, 8, null)).toEqual({ dir: 'NE', steps: 3 });
    expect(snapSelection(A, ...at(-3 * c, 3 * c), c, 8, 8, null)).toEqual({ dir: 'SW', steps: 3 });
    expect(snapSelection(A, ...at(2.9 * c, 2.9 * c), c, 8, 8, null)).toEqual({ dir: 'SE', steps: 3 });
  });
  it('clamps at the edges', () => {
    expect(snapSelection({ row: 0, col: 0 }, 0.5 * c, -3 * c, c, 8, 8, null)).toEqual({ dir: 'N', steps: 0 });
    expect(snapSelection(A, ...at(20 * c, 0), c, 8, 8, null)).toEqual({ dir: 'E', steps: 4 });
  });
  it('has hysteresis', () => {
    expect(snapSelection(A, ...deg(28), c, 8, 8, 'E').dir).toBe('E');
    expect(snapSelection(A, ...deg(40), c, 8, 8, 'E').dir).toBe('SE');
    expect(snapSelection(A, ...deg(28), c, 8, 8, null).dir).toBe('SE');
  });
});

describe('matchWord', () => {
  it('matches both directions', () => {
    expect(matchWord('OLEG', new Set(['GELO']))).toBe('GELO');
    expect(matchWord('GELO', new Set(['GELO']))).toBe('GELO');
    expect(matchWord('ABCD', new Set(['GELO']))).toBe(null);
  });
});
