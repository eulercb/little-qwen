import { describe, expect, it } from 'vitest';
import { defaultSave, parseSave, serializeSave } from './save';

describe('save', () => {
  it('falls back to the default', () => {
    expect(parseSave(null)).toEqual(defaultSave());
    expect(parseSave('{nope')).toEqual(defaultSave());
    expect(parseSave(JSON.stringify({ v: 2 }))).toEqual(defaultSave());
    const bad = { ...defaultSave(), progress: { key: 'praia:1', found: 'X' } };
    expect(parseSave(JSON.stringify(bad))).toEqual(defaultSave());
  });
  it('round-trips and drops unknown fields', () => {
    const s = { v: 1 as const, solved: ['praia:1'], progress: { key: 'praia:2', found: ['ONDA'] }, settings: { haptics: false, theme: 'dark' as const } };
    expect(parseSave(serializeSave(s))).toEqual(s);
    expect(parseSave(JSON.stringify({ ...s, extra: 1 }))).toEqual(s);
  });
});
