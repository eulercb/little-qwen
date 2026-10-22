import { SAVE_KEY, defaultSave, parseSave, serializeSave } from '../core/save';
import type { SaveState } from '../core/types';

let state: SaveState | null = null;

export function loadSave(): SaveState {
  try {
    state = parseSave(localStorage.getItem(SAVE_KEY));
  } catch {
    state = defaultSave();
  }
  return state;
}

/** The single in-memory save the UI mutates. */
export function getSave(): SaveState {
  return state ?? loadSave();
}

export function saveState(s: SaveState = getSave()): void {
  state = s;
  try {
    localStorage.setItem(SAVE_KEY, serializeSave(s));
  } catch {
    // Storage may be full or disabled; the game keeps working in memory.
  }
}
