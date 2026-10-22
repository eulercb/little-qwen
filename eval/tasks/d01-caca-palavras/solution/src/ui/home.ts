import { PUZZLES_PER_THEME, puzzleKey } from '../core/levels';
import { THEMES, themeById } from '../data/words';
import { navigate } from './router';
import { openSettings } from './settings';
import { getSave } from './storage';
import { STR } from './strings';

function span(cls: string | null, text: string): HTMLSpanElement {
  const s = document.createElement('span');
  if (cls) s.className = cls;
  s.textContent = text;
  return s;
}

function themeText(title: string, sub: string): HTMLSpanElement {
  const t = span('theme-text', '');
  const strong = document.createElement('strong');
  strong.textContent = title;
  t.append(strong, span(null, sub));
  return t;
}

export function renderHome(app: HTMLElement): void {
  const save = getSave();
  const head = document.createElement('header');
  head.className = 'home-head';
  const h1 = document.createElement('h1');
  h1.textContent = STR.appName;
  const tagline = document.createElement('p');
  tagline.className = 'tagline';
  tagline.textContent = STR.tagline;
  const gear = document.createElement('button');
  gear.className = 'icon-btn';
  gear.setAttribute('aria-label', STR.settings);
  gear.textContent = '⚙';
  gear.addEventListener('click', openSettings);
  head.append(h1, tagline, gear);

  const main = document.createElement('main');
  main.className = 'themes';
  const progress = save.progress;
  if (progress !== null) {
    const [id, idx] = progress.key.split(':');
    const theme = themeById(id);
    if (theme) {
      const row = document.createElement('button');
      row.className = 'theme-row continue';
      row.append(span('emoji', theme.emoji), themeText(STR.continue, STR.continueSub(theme.name, Number(idx))));
      row.addEventListener('click', () => navigate(`#/p/${id}/${idx}`));
      main.append(row);
    }
  }
  for (const theme of THEMES) {
    const solved = Array.from({ length: PUZZLES_PER_THEME }, (_, i) => save.solved.includes(puzzleKey(theme.id, i + 1)));
    const done = solved.filter(Boolean).length;
    const row = document.createElement('button');
    row.className = 'theme-row';
    row.dataset.theme = theme.id;
    row.append(span('emoji', theme.emoji), themeText(theme.name, STR.progress(done, PUZZLES_PER_THEME)));
    if (done === PUZZLES_PER_THEME) {
      const check = span('check', '✓');
      check.setAttribute('aria-label', STR.themeDone);
      row.append(check);
    } else {
      const bar = span('bar', '');
      const fill = span('bar-fill', '');
      fill.style.width = `${Math.round((done / PUZZLES_PER_THEME) * 100)}%`;
      bar.append(fill);
      row.append(bar);
    }
    const next = solved.indexOf(false) + 1 || 1;
    row.addEventListener('click', () => navigate(`#/p/${theme.id}/${next}`));
    main.append(row);
  }
  app.replaceChildren(head, main);
}
