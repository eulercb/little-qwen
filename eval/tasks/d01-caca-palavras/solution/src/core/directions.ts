import type { Cell, Dir } from './types';

export const DIRS: Record<Dir, readonly [number, number]> = {
  E: [0, 1], SE: [1, 1], S: [1, 0], SW: [1, -1], W: [0, -1], NW: [-1, -1], N: [-1, 0], NE: [-1, 1],
};
// Clockwise from E in 45° steps. Index i ↔ angle i·π/4 measured with atan2(dy, dx) in screen coords.
export const DIR_ORDER: Dir[] = ['E', 'SE', 'S', 'SW', 'W', 'NW', 'N', 'NE'];
export function isDiagonal(d: Dir): boolean { const [dr, dc] = DIRS[d]; return dr !== 0 && dc !== 0; }
export function angleOf(d: Dir): number { return DIR_ORDER.indexOf(d) * Math.PI / 4; } // 0..7π/4

function axisSteps(pos: number, delta: number, size: number): number {
  if (delta > 0) return size - 1 - pos;
  if (delta < 0) return pos;
  return Infinity;
}

/** Number of steps from `from` in direction d that stay inside a rows×cols grid. */
export function maxSteps(from: Cell, d: Dir, rows: number, cols: number): number {
  const [dr, dc] = DIRS[d];
  return Math.max(0, Math.min(axisSteps(from.row, dr, rows), axisSteps(from.col, dc, cols)));
}

/** Cells from `from`, `steps` steps along d, inclusive (length steps+1). */
export function pathCells(from: Cell, d: Dir, steps: number): Cell[] {
  const [dr, dc] = DIRS[d];
  const out: Cell[] = [];
  for (let i = 0; i <= steps; i++) out.push({ row: from.row + dr * i, col: from.col + dc * i });
  return out;
}
