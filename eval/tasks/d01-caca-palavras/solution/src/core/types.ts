export type Dir = 'E' | 'SE' | 'S' | 'SW' | 'W' | 'NW' | 'N' | 'NE';

export interface Cell { row: number; col: number }

export interface Theme {
  id: string;        // slug, e.g. 'praia'
  name: string;      // display, e.g. 'Praia'
  emoji: string;     // one emoji for the home list
  words: string[];   // display forms with accents, e.g. 'Maracujá'
}

export interface TierSpec {
  tier: 1 | 2 | 3 | 4;
  name: string;      // 'Suave' | 'Tranquilo' | 'Atento' | 'Sereno'
  rows: number;
  cols: number;
  wordCount: number;
  minLen: number;
  maxLen: number;
  directions: Dir[]; // may contain repeats to weight a direction
}

export interface Placement {
  word: string;      // normalized, e.g. 'MARACUJA'
  display: string;   // 'Maracujá'
  row: number;       // first letter
  col: number;
  dir: Dir;
}

export interface Puzzle {
  key: string;       // `${themeId}:${index}`
  themeId: string;
  index: number;     // 1..12
  tier: 1 | 2 | 3 | 4;
  rows: number;
  cols: number;
  grid: string[];    // rows*cols single uppercase letters, row-major
  placements: Placement[];
}

export interface FoundWord { word: string; cells: Cell[]; colorIndex: number }

export interface Settings { haptics: boolean; theme: 'auto' | 'light' | 'dark' }

export interface SaveState {
  v: 1;
  solved: string[];                                  // puzzle keys
  progress: { key: string; found: string[] } | null; // normalized words found in the open puzzle
  settings: Settings;
}
