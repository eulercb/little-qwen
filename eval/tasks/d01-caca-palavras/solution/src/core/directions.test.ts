import { describe, expect, it } from 'vitest';
import { angleOf, isDiagonal, maxSteps, pathCells } from './directions';

describe('directions', () => {
  it('maxSteps', () => {
    expect(maxSteps({ row: 0, col: 0 }, 'E', 8, 8)).toBe(7);
    expect(maxSteps({ row: 0, col: 0 }, 'N', 8, 8)).toBe(0);
    expect(maxSteps({ row: 2, col: 3 }, 'SW', 8, 8)).toBe(3);
  });
  it('pathCells', () => {
    expect(pathCells({ row: 0, col: 0 }, 'SE', 2)).toEqual([{ row: 0, col: 0 }, { row: 1, col: 1 }, { row: 2, col: 2 }]);
  });
  it('angles and diagonals', () => {
    expect(angleOf('S')).toBe(Math.PI / 2);
    expect(isDiagonal('NE') && !isDiagonal('W')).toBe(true);
  });
});
