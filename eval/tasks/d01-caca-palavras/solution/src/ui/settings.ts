import type { Settings } from '../core/types';
import { getSave, saveState } from './storage';
import { STR } from './strings';

let media: MediaQueryList | null = null;

/** Apply settings.theme to <html data-theme> and the theme-color meta. */
export function applyTheme(): void {
  const pref = getSave().settings.theme;
  media ??= window.matchMedia('(prefers-color-scheme: dark)');
  const mode = pref === 'auto' ? (media.matches ? 'dark' : 'light') : pref;
  document.documentElement.dataset.theme = mode;
  const bg = getComputedStyle(document.documentElement).getPropertyValue('--bg').trim();
  document.querySelector('meta[name="theme-color"]')?.setAttribute('content', bg || '#EEF1EA');
}

export function watchSystemTheme(): void {
  media ??= window.matchMedia('(prefers-color-scheme: dark)');
  media.addEventListener('change', () => {
    if (getSave().settings.theme === 'auto') applyTheme();
  });
}

export function openSettings(): void {
  const save = getSave();
  const backdrop = document.createElement('div');
  backdrop.className = 'sheet-backdrop';
  const sheet = document.createElement('div');
  sheet.className = 'sheet';
  sheet.setAttribute('role', 'dialog');

  const hapticsRow = document.createElement('label');
  hapticsRow.className = 'setting-row';
  const hText = document.createElement('span');
  hText.textContent = STR.haptics;
  const toggle = document.createElement('input');
  toggle.type = 'checkbox';
  toggle.className = 'switch';
  toggle.checked = save.settings.haptics;
  toggle.addEventListener('change', () => {
    save.settings.haptics = toggle.checked;
    saveState(save);
  });
  hapticsRow.append(hText, toggle);

  const themeRow = document.createElement('div');
  themeRow.className = 'setting-row';
  const tText = document.createElement('span');
  tText.textContent = STR.appearance;
  const seg = document.createElement('div');
  seg.className = 'segmented';
  const options: [Settings['theme'], string][] = [['auto', STR.themeAuto], ['light', STR.themeLight], ['dark', STR.themeDark]];
  const buttons = options.map(([value, label]) => {
    const b = document.createElement('button');
    b.textContent = label;
    b.setAttribute('aria-pressed', String(save.settings.theme === value));
    b.addEventListener('click', () => {
      save.settings.theme = value;
      saveState(save);
      applyTheme();
      buttons.forEach((x, i) => x.setAttribute('aria-pressed', String(options[i][0] === value)));
    });
    return b;
  });
  seg.append(...buttons);
  themeRow.append(tText, seg);

  const close = document.createElement('button');
  close.className = 'btn secondary';
  close.textContent = STR.close;
  const dismiss = () => backdrop.remove();
  close.addEventListener('click', dismiss);
  backdrop.addEventListener('click', (e) => { if (e.target === backdrop) dismiss(); });
  sheet.append(hapticsRow, themeRow, close);
  backdrop.append(sheet);
  document.body.append(backdrop);
  window.addEventListener('hashchange', dismiss, { once: true });
}
