import type { Cell, FoundWord, Puzzle } from '../core/types';

const SVG = 'http://www.w3.org/2000/svg';
const PAPER_PAD = 8;

export interface GridView {
  grid: HTMLDivElement;
  cells: HTMLDivElement[];
  active: SVGLineElement;
  cellSize: () => number;
  setLine: (line: SVGLineElement, from: Cell, to: Cell) => void;
  addFound: (f: FoundWord) => SVGLineElement;
  cellEl: (c: Cell) => HTMLDivElement;
}

export function renderGrid(area: HTMLElement, puzzle: Puzzle, getFound: () => FoundWord[]): GridView {
  const paper = document.createElement('div');
  paper.className = 'grid-paper';
  const grid = document.createElement('div');
  grid.className = 'grid';
  grid.setAttribute('role', 'grid');
  const svg = document.createElementNS(SVG, 'svg');
  svg.setAttribute('class', 'overlay');
  svg.setAttribute('aria-hidden', 'true');
  const foundG = document.createElementNS(SVG, 'g');
  foundG.setAttribute('class', 'found');
  const active = document.createElementNS(SVG, 'line');
  active.setAttribute('class', 'active');
  active.setAttribute('stroke-linecap', 'round');
  active.style.display = 'none';
  svg.append(foundG, active);
  const cellsWrap = document.createElement('div');
  cellsWrap.className = 'cells';
  const cells: HTMLDivElement[] = [];
  puzzle.grid.forEach((letter, i) => {
    const d = document.createElement('div');
    d.className = 'cell';
    d.dataset.r = String(Math.floor(i / puzzle.cols));
    d.dataset.c = String(i % puzzle.cols);
    d.textContent = letter;
    cells.push(d);
  });
  cellsWrap.append(...cells);
  grid.append(svg, cellsWrap);
  paper.append(grid);
  area.append(paper);

  let cell = 40;
  const center = (c: Cell) => [(c.col + 0.5) * cell, (c.row + 0.5) * cell];
  const setLine = (line: SVGLineElement, from: Cell, to: Cell) => {
    const [x1, y1] = center(from);
    const [x2, y2] = center(to);
    line.setAttribute('x1', String(x1));
    line.setAttribute('y1', String(y1));
    line.setAttribute('x2', String(x2));
    line.setAttribute('y2', String(y2));
    line.setAttribute('stroke-width', String(cell * 0.78));
  };
  const addFound = (f: FoundWord) => {
    const line = document.createElementNS(SVG, 'line');
    line.setAttribute('stroke-linecap', 'round');
    line.style.stroke = `var(--p${f.colorIndex})`;
    line.style.opacity = 'var(--pill-alpha)';
    setLine(line, f.cells[0], f.cells[f.cells.length - 1]);
    foundG.append(line);
    return line;
  };
  const layout = () => {
    const availW = area.clientWidth - 24 - PAPER_PAD * 2;
    const availH = area.clientHeight - 12 - PAPER_PAD * 2;
    cell = Math.min(56, Math.max(28, Math.floor(Math.min(availW / puzzle.cols, availH / puzzle.rows))));
    const w = puzzle.cols * cell;
    const h = puzzle.rows * cell;
    grid.style.setProperty('--cell', `${cell}px`);
    grid.style.setProperty('--cols', String(puzzle.cols));
    grid.style.width = `${w}px`;
    grid.style.height = `${h}px`;
    svg.setAttribute('width', String(w));
    svg.setAttribute('height', String(h));
    svg.setAttribute('viewBox', `0 0 ${w} ${h}`);
    foundG.replaceChildren();
    for (const f of getFound()) addFound(f);
  };
  new ResizeObserver(layout).observe(area);
  layout();
  return {
    grid, cells, active, setLine, addFound,
    cellSize: () => cell,
    cellEl: (c: Cell) => cells[c.row * puzzle.cols + c.col],
  };
}
