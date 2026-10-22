import { letterAt } from '../core/generate';
import { matchWord, selectionCells, snapSelection } from '../core/select';
import type { Cell, Dir, Puzzle } from '../core/types';
import type { GridView } from './grid';

export interface PointerHooks {
  remaining: () => Set<string>;
  onFound: (word: string, cells: Cell[]) => void;
  onTrace: (letters: string | null) => void;
}

interface Drag { pointerId: number; anchor: Cell; prevDir: Dir | null; cells: Cell[] }

export function attachPointer(view: GridView, puzzle: Puzzle, hooks: PointerHooks): void {
  const { grid, active } = view;
  let drag: Drag | null = null;
  const local = (e: PointerEvent) => {
    const r = grid.getBoundingClientRect();
    return [e.clientX - r.left, e.clientY - r.top];
  };
  const letters = (cells: Cell[]) => cells.map((c) => letterAt(puzzle, c)).join('');
  const show = (cells: Cell[]) => {
    active.style.transition = '';
    active.style.opacity = '';
    active.style.display = '';
    view.setLine(active, cells[0], cells[cells.length - 1]);
    hooks.onTrace(letters(cells));
  };

  grid.addEventListener('pointerdown', (e) => {
    if (e.pointerType === 'mouse' && e.button !== 0) return;
    const [px, py] = local(e);
    const cell = view.cellSize();
    const anchor = { row: Math.floor(py / cell), col: Math.floor(px / cell) };
    if (anchor.row < 0 || anchor.col < 0 || anchor.row >= puzzle.rows || anchor.col >= puzzle.cols) return;
    grid.setPointerCapture(e.pointerId);
    e.preventDefault();
    drag = { pointerId: e.pointerId, anchor, prevDir: null, cells: [anchor] };
    show(drag.cells);
  });

  grid.addEventListener('pointermove', (e) => {
    if (!drag || e.pointerId !== drag.pointerId) return;
    const [px, py] = local(e);
    const snap = snapSelection(drag.anchor, px, py, view.cellSize(), puzzle.rows, puzzle.cols, drag.prevDir);
    drag.prevDir = snap.dir;
    const cells = selectionCells(drag.anchor, snap);
    const same = cells.length === drag.cells.length
      && cells.every((c, i) => c.row === drag?.cells[i].row && c.col === drag?.cells[i].col);
    if (same) return;
    drag.cells = cells;
    show(cells);
  });

  const finish = (e: PointerEvent) => {
    if (!drag || e.pointerId !== drag.pointerId) return;
    const { cells } = drag;
    drag = null;
    const word = matchWord(letters(cells), hooks.remaining());
    if (word) {
      active.style.display = 'none';
      hooks.onFound(word, cells);
      return;
    }
    hooks.onTrace(null);
    active.style.transition = 'opacity 180ms ease-out';
    active.style.opacity = '0';
    window.setTimeout(() => {
      if (!drag) active.style.display = 'none';
    }, 180);
  };
  grid.addEventListener('pointerup', finish);
  grid.addEventListener('pointercancel', finish);
  grid.addEventListener('lostpointercapture', finish);
}
