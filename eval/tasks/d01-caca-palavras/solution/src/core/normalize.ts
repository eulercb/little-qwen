export function normalizeWord(s: string): string {
  return s.normalize('NFD').replace(/\p{M}/gu, '').toUpperCase();
}
export function isValidWord(s: string): boolean {
  const n = normalizeWord(s);
  return /^[A-Z]{4,9}$/.test(n);
}
export function reverse(s: string): string { return [...s].reverse().join(''); }
