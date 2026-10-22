import { PUZZLES_PER_THEME } from '../core/levels';
import { themeById } from '../data/words';

export type Route = { name: 'home' } | { name: 'puzzle'; themeId: string; index: number };

export function parseHash(hash: string = location.hash): Route {
  const m = /^#\/p\/([^/]+)\/(\d+)$/.exec(hash);
  if (m) {
    const index = Number(m[2]);
    if (themeById(m[1]) && Number.isInteger(index) && index >= 1 && index <= PUZZLES_PER_THEME) {
      return { name: 'puzzle', themeId: m[1], index };
    }
  }
  return { name: 'home' };
}

export function navigate(hash: string): void {
  location.hash = hash;
}

export function onRoute(cb: (route: Route) => void): void {
  window.addEventListener('hashchange', () => cb(parseHash()));
  cb(parseHash());
}
