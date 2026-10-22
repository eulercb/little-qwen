import type { SaveState, Settings } from './types';

export const SAVE_KEY = 'cp.save.v1';

export function defaultSave(): SaveState {
  return { v: 1, solved: [], progress: null, settings: { haptics: true, theme: 'auto' } };
}

function isStringArray(x: unknown): x is string[] {
  return Array.isArray(x) && x.every((s) => typeof s === 'string');
}

function parseSettings(x: unknown): Settings | null {
  if (typeof x !== 'object' || x === null) return null;
  const o = x as Record<string, unknown>;
  if (typeof o.haptics !== 'boolean') return null;
  if (o.theme !== 'auto' && o.theme !== 'light' && o.theme !== 'dark') return null;
  return { haptics: o.haptics, theme: o.theme };
}

export function parseSave(json: string | null): SaveState {
  if (json === null) return defaultSave();
  let raw: unknown;
  try {
    raw = JSON.parse(json);
  } catch {
    return defaultSave();
  }
  if (typeof raw !== 'object' || raw === null) return defaultSave();
  const o = raw as Record<string, unknown>;
  if (o.v !== 1 || !isStringArray(o.solved)) return defaultSave();
  const settings = parseSettings(o.settings);
  if (!settings) return defaultSave();
  let progress: SaveState['progress'] = null;
  if (o.progress !== null) {
    if (typeof o.progress !== 'object' || o.progress === undefined) return defaultSave();
    const p = o.progress as Record<string, unknown>;
    if (typeof p.key !== 'string' || !isStringArray(p.found)) return defaultSave();
    progress = { key: p.key, found: [...p.found] };
  }
  return { v: 1, solved: [...o.solved], progress, settings };
}

export function serializeSave(s: SaveState): string {
  return JSON.stringify(s);
}
