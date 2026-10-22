import type { TierSpec } from './types';

export const PUZZLES_PER_THEME = 12;
export const TIERS: TierSpec[] = [
  { tier: 1, name: 'Suave',     rows: 8,  cols: 8,  wordCount: 5, minLen: 4, maxLen: 7, directions: ['E', 'S'] },
  { tier: 2, name: 'Tranquilo', rows: 10, cols: 9,  wordCount: 6, minLen: 4, maxLen: 8, directions: ['E', 'S', 'SE', 'NE'] },
  { tier: 3, name: 'Atento',    rows: 11, cols: 10, wordCount: 7, minLen: 4, maxLen: 9, directions: ['E', 'E', 'S', 'S', 'SE', 'NE', 'W', 'N'] },
  { tier: 4, name: 'Sereno',    rows: 13, cols: 10, wordCount: 8, minLen: 4, maxLen: 9, directions: ['E', 'S', 'SE', 'NE', 'W', 'N', 'NW', 'SW'] },
];
export function tierForIndex(index: number): TierSpec { return TIERS[Math.ceil(index / 3) - 1]; } // 1-3→1, 4-6→2, ...
export function puzzleKey(themeId: string, index: number): string { return `${themeId}:${index}`; }
