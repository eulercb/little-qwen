import { describe, expect, it } from 'vitest';
import { isValidWord, normalizeWord, reverse } from './normalize';

describe('normalize', () => {
  it('strips accents and cedilla', () => {
    expect(normalizeWord('Açúcar')).toBe('ACUCAR');
    expect(normalizeWord('Maracujá')).toBe('MARACUJA');
    expect(normalizeWord('Pêssego')).toBe('PESSEGO');
    expect(normalizeWord('Folião')).toBe('FOLIAO');
  });
  it('validates words', () => {
    expect(isValidWord('Uva')).toBe(false);
    expect(isValidWord('Jabuticaba')).toBe(false);
    expect(isValidWord('Guarda-sol')).toBe(false);
    expect(isValidWord('Pão de queijo')).toBe(false);
    expect(isValidWord('Maçã')).toBe(true);
  });
  it('reverses', () => { expect(reverse('ABC')).toBe('CBA'); });
});
