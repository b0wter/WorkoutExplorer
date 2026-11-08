// Pure rules of the upload page (F-11 design U-5 to U-9). AC 1 (edit link), AC 2/6 (size).
import { describe, expect, it } from 'vitest';
import { canPick, editTarget, outcomeOf, parseStatus, percent, SENTENCES, tooLarge } from './rules';

const ORIGIN = 'https://workouts.example';
const ID = '4kQ0dTqv1Zr8mN2xYbL7Ce';
const TOKEN = 'P9sTa0Wm3Hq5Lx2Kd8Rf1V';
const EDIT = `/edit/${ID}#${TOKEN}`;

describe('tooLarge (AC 2, AC 6)', () => {
  it('sends exactly 20,000,000 bytes and refuses 20,000,001', () => {
    expect(tooLarge(20_000_000)).toBe(false);
    expect(tooLarge(20_000_001)).toBe(true);
    expect(tooLarge(25_000_000)).toBe(true);
    expect(tooLarge(0)).toBe(false);
  });
});

describe('editTarget (AC 1)', () => {
  it('resolves against the origin and adds nothing', () => {
    expect(editTarget(EDIT, ORIGIN)).toBe(`${ORIGIN}${EDIT}`);
  });
  it.each([
    ['other origin', `https://evil.example/edit/${ID}#${TOKEN}`],
    ['protocol-relative', `//evil.example/edit/${ID}#${TOKEN}`],
    ['wrong path', `/view/${ID}#${TOKEN}`],
    ['short id', `/edit/abc#${TOKEN}`],
    ['long id', `/edit/${ID}x#${TOKEN}`],
    ['no fragment', `/edit/${ID}`],
    ['short token', `/edit/${ID}#abc`],
    ['token with punctuation', `/edit/${ID}#${TOKEN.slice(0, 21)}-`],
    ['query', `/edit/${ID}?a=b#${TOKEN}`],
    ['extra segment', `/edit/${ID}/x#${TOKEN}`],
    ['not a string', 42],
    ['missing', undefined],
  ])('refuses %s', (_n, v) => {
    expect(editTarget(v, ORIGIN)).toBeNull();
  });
});

describe('outcomeOf (U-8, U-9)', () => {
  const json = (o: unknown) => JSON.stringify(o);
  it('201 with a valid edit_url navigates', () => {
    expect(outcomeOf(201, json({ view_url: '/w/x', edit_url: EDIT }), ORIGIN)).toEqual({
      kind: 'navigate',
      url: `${ORIGIN}${EDIT}`,
    });
  });
  it('201 with a bad link or a non-JSON body is a server error', () => {
    const bad = { kind: 'error', message: SENTENCES.serverError };
    expect(outcomeOf(201, json({ edit_url: 'https://evil.example/edit/x' }), ORIGIN)).toEqual(bad);
    expect(outcomeOf(201, '<html>', ORIGIN)).toEqual(bad);
    expect(outcomeOf(201, json({}), ORIGIN)).toEqual(bad);
  });
  it.each([
    [422, 'not_fit', 'This file is not a FIT file.'],
    [422, 'unsupported_type', 'Unsupported FIT file type.'],
    [429, 'rate_limited', 'Too many uploads. Please try again later.'],
    [503, 'busy', 'Too many uploads right now. Please try again in a minute.'],
    [413, 'too_large', 'This file is larger than 20 MB.'],
  ])('%i %s shows the server message as it is (AC 3, AC 5)', (status, error, message) => {
    expect(outcomeOf(status, json({ error, message }), ORIGIN)).toEqual({ kind: 'error', message });
  });
  it('503 storage_full switches to paused', () => {
    expect(outcomeOf(503, json({ error: 'storage_full', message: SENTENCES.paused }), ORIGIN)).toEqual({
      kind: 'paused',
      message: SENTENCES.paused,
    });
  });
  it('a proxy page, an empty body or a missing message gives the server error', () => {
    const err = { kind: 'error', message: SENTENCES.serverError };
    expect(outcomeOf(502, '<html>Bad gateway</html>', ORIGIN)).toEqual(err);
    expect(outcomeOf(500, '', ORIGIN)).toEqual(err);
    expect(outcomeOf(500, json({ error: 'x', message: '' }), ORIGIN)).toEqual(err);
    expect(outcomeOf(500, json({ error: 'x', message: 5 }), ORIGIN)).toEqual(err);
  });
});

describe('parseStatus (U-5)', () => {
  it('reads the flag and rejects anything else', () => {
    expect(parseStatus('{"uploads_open":true}')).toBe(true);
    expect(parseStatus('{"uploads_open":false}')).toBe(false);
    expect(parseStatus('{"uploads_open":"no"}')).toBeNull();
    expect(parseStatus('{}')).toBeNull();
    expect(parseStatus('nope')).toBeNull();
    expect(parseStatus('null')).toBeNull();
  });
});

describe('percent and canPick', () => {
  it('rounds down and clamps', () => {
    expect(percent(42, 100)).toBe(42);
    expect(percent(1, 3)).toBe(33);
    expect(percent(5, 0)).toBe(0);
    expect(percent(200, 100)).toBe(100);
  });
  it('only the ready state accepts a file', () => {
    expect(canPick('ready')).toBe(true);
    for (const s of ['checking', 'opening', 'uploading', 'processing', 'paused'] as const) expect(canPick(s)).toBe(false);
  });
});
