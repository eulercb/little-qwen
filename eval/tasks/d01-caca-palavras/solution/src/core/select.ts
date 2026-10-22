import { DIR_ORDER, angleOf, isDiagonal, maxSteps, pathCells } from './directions';
import { reverse } from './normalize';
import type { Cell, Dir } from './types';

export interface Snap { dir: Dir | null; steps: number }

function wrap(a: number): number {
  let x = a;
  while (x <= -Math.PI) x += 2 * Math.PI;
  while (x > Math.PI) x -= 2 * Math.PI;
  return x;
}

export function snapSelection(
  anchor: Cell, px: number, py: number, cell: number,
  rows: number, cols: number, prevDir: Dir | null,
): Snap {
  const ax = (anchor.col + 0.5) * cell;
  const ay = (anchor.row + 0.5) * cell;
  const dx = px - ax;
  const dy = py - ay;
  const dist = Math.hypot(dx, dy);
  if (dist < 0.45 * cell) return { dir: prevDir, steps: 0 };
  const angle = Math.atan2(dy, dx);
  let dir: Dir;
  if (prevDir !== null && Math.abs(wrap(angle - angleOf(prevDir))) <= (3 * Math.PI) / 16) {
    dir = prevDir;
  } else {
    dir = DIR_ORDER[((Math.round(angle / (Math.PI / 4)) % 8) + 8) % 8];
  }
  const unit = isDiagonal(dir) ? cell * Math.SQRT2 : cell;
  const steps = Math.min(Math.max(Math.round(dist / unit), 0), maxSteps(anchor, dir, rows, cols));
  return { dir, steps };
}

export function selectionCells(anchor: Cell, snap: Snap): Cell[] {
  if (snap.dir === null || snap.steps === 0) return [anchor];
  return pathCells(anchor, snap.dir, snap.steps);
}

export function matchWord(letters: string, remaining: Set<string>): string | null {
  if (remaining.has(letters)) return letters;
  const r = reverse(letters);
  if (remaining.has(r)) return r;
  return null;
}
