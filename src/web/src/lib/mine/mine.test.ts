// lib/mine (F-53 MW-2 to MW-7, ADR-058). Critical: stored values are hostile input; links come
// only from checked parts; every storage failure is contained; an unreadable value is never rewritten.
import { describe, expect, it } from 'vitest';
import { LIST_KEY, clear, effectiveExpiry, forget, prune, remove, toRow, update, upsert, type ListStore } from '.';

const ID = 'AbCdEfGhIjKlMnOpQrStUv';
const TOKEN = 'Zz0123456789abcdefghij';
const OTHER = 'Yy0123456789abcdefghij';
const H = 3600_000;
const NOW = Date.parse('2026-11-20T12:00:00Z');
const iso = (ms: number) => new Date(ms).toISOString();

function memory(init?: string) {
  const m = new Map<string, string>(init === undefined ? [] : [[LIST_KEY, init]]);
  const s: ListStore & { writes: number } = {
    writes: 0,
    getItem: (k) => m.get(k) ?? null,
    setItem: (k, v) => void (s.writes++, m.set(k, v)),
    removeItem: (k) => void m.delete(k),
  };
  return { s, get: () => s, value: () => m.get(LIST_KEY) };
}
const list = (...workouts: unknown[]) => JSON.stringify({ v: 1, workouts });
const entry = (over: Record<string, unknown> = {}) => ({ id: ID, token: TOKEN, title: 't', sport: 'running', date: '2026-11-19', created: iso(NOW - H), expires: iso(NOW + 30 * 24 * H), published: false, ...over });

describe('effective expiry (MW-4)', () => {
  it('published: expires; a draft: the earlier of expires and created + 48 h; unparseable: NaN', () => {
    expect(effectiveExpiry(iso(NOW), iso(NOW + 100 * H), true)).toBe(NOW + 100 * H);
    expect(effectiveExpiry(iso(NOW), iso(NOW + 100 * H), false)).toBe(NOW + 48 * H);
    expect(effectiveExpiry(iso(NOW), iso(NOW + 24 * H), false)).toBe(NOW + 24 * H);
    expect(effectiveExpiry(iso(NOW), iso(NOW + 100 * H), 'true')).toBe(NOW + 48 * H); // wrong type: a draft
    for (const [c, e] of [['x', iso(NOW)], [iso(NOW), 5], [undefined, iso(NOW)]]) expect(effectiveExpiry(c, e, true)).toBeNaN();
  });
});

describe('rows (MW-5): every field checked on its own, links only from checked parts', () => {
  it('a good entry: both links root-relative, view only when published', () => {
    expect(toRow(entry())).toMatchObject({ title: 't', sport: 'running', date: '2026-11-19', edit: '/edit/' + ID + '#' + TOKEN, view: undefined });
    expect(toRow(entry({ published: true })).view).toBe('/w/' + ID);
  });

  it.each<unknown>(['javascript:alert(1)', 'https://evil.example/w/' + ID, '//evil.example', ID + '/../x', ID.slice(1) + '-', 42])('a bad id (%s): neither Edit nor View', (id) => {
    const r = toRow(entry({ id, published: true }));
    expect([r.edit, r.view]).toEqual([undefined, undefined]);
  });

  it.each(['javascript:alert(1)', TOKEN + '&x', TOKEN.slice(1), null, 'https://evil.example/edit/' + ID + '#' + TOKEN])('a bad token (%s): no Edit, View stays', (token) => {
    const r = toRow(entry({ token, published: true }));
    expect([r.edit, r.view]).toEqual([undefined, '/w/' + ID]);
  });

  it('a stored url field is never used', () => {
    const r = toRow(entry({ url: 'javascript:alert(1)', edit: 'javascript:1', view: '//evil' }));
    expect(r.edit).toBe('/edit/' + ID + '#' + TOKEN);
    expect(r.view).toBeUndefined();
  });

  it('wrong types and formats are absent, never repaired', () => {
    const r = toRow(entry({ title: ['<b>'], sport: 7, date: '2026-13-45' }));
    expect([r.title, r.sport, r.date]).toEqual([undefined, undefined, undefined]);
    expect(toRow(entry({ date: '19 Nov 2026' })).date).toBeUndefined();
  });
});

describe('prune (MW-4, MW-6)', () => {
  it('drops expired and unparseable entries, writes back, newest created first', () => {
    const m = memory(
      list(
        entry({ id: 'A'.repeat(22), created: iso(NOW - 3 * H) }),
        entry({ id: 'B'.repeat(22), created: iso(NOW - H) }),
        entry({ id: 'C'.repeat(22), created: iso(NOW - 49 * H) }), // draft past 48 h
        entry({ id: 'D'.repeat(22), created: iso(NOW - 49 * H), published: true }),
        entry({ id: 'E'.repeat(22), expires: 'soon' }),
        entry({ id: 'F'.repeat(22), expires: iso(NOW) }), // the device clock decides, no grace
        'a string',
        null,
        [1],
      ),
    );
    const rows = prune(m.get, NOW)!;
    expect(rows.map((r) => r.edit?.slice(6, 7))).toEqual(['B', 'A', 'D']);
    expect(JSON.parse(m.value()!).workouts.map((e: { id: string }) => e.id[0])).toEqual(['A', 'B', 'D']);
  });

  it('no key: an empty, available list', () => {
    expect(prune(memory().get, NOW)).toEqual([]);
  });

  it.each(['{', '[]', '{"v":2,"workouts":[]}', '{"v":"1","workouts":[]}', '{"v":1}', '{"v":1,"workouts":{}}', 'null', '{"v":1,"workouts":[{"id":"'])('unreadable (%s): not available, never rewritten', (raw) => {
    const m = memory(raw);
    expect(prune(m.get, NOW)).toBeNull();
    expect(m.value()).toBe(raw);
    expect(m.s.writes).toBe(0);
  });

  it('storage that reads but refuses writes, or is missing or throws: not available', () => {
    const m = memory(list(entry()));
    m.s.setItem = () => {
      throw new DOMException('full', 'QuotaExceededError');
    };
    expect(prune(m.get, NOW)).toBeNull();
    expect(prune(() => null, NOW)).toBeNull();
    expect(
      prune(() => {
        throw new DOMException('blocked', 'SecurityError');
      }, NOW),
    ).toBeNull();
    expect(prune(() => ({ ...m.s, getItem: () => { throw new Error('x'); } }), NOW)).toBeNull();
  });
});

describe('writers (MW-3)', () => {
  const e = { id: ID, token: TOKEN, title: 'A', sport: 'cycling', date: '2026-11-19', created: iso(NOW), expires: iso(NOW + H), published: false };

  it('upsert adds, then replaces by id (one entry per workout); absent sport and date keep the stored ones', () => {
    const m = memory(list(entry({ id: 'B'.repeat(22) })));
    upsert(m.get, e, NOW);
    upsert(m.get, { ...e, title: 'B', sport: undefined, date: undefined, published: true }, NOW);
    const w = JSON.parse(m.value()!).workouts;
    expect(w.map((x: { id: string }) => x.id)).toEqual(['B'.repeat(22), ID]);
    expect(w[1]).toEqual({ ...e, title: 'B', published: true });
  });

  it('upsert drops other expired entries (About), keeps unparseable ones', () => {
    const m = memory(list(entry({ id: 'B'.repeat(22), published: true, expires: iso(NOW - 1) }), entry({ id: 'C'.repeat(22), expires: 5 })));
    upsert(m.get, e, NOW);
    expect(JSON.parse(m.value()!).workouts.map((x: { id: string }) => x.id)).toEqual(['C'.repeat(22), ID]);
  });

  it('upsert replaces an unreadable value with a fresh list (ADR-058 item 4)', () => {
    const m = memory('garbage');
    upsert(m.get, e);
    expect(JSON.parse(m.value()!)).toEqual({ v: 1, workouts: [e] });
  });

  it('update changes an existing entry only, never adds', () => {
    const m = memory(list());
    update(m.get, ID, { title: 'x', created: e.created, expires: e.expires, published: true });
    expect(JSON.parse(m.value()!).workouts).toEqual([]);
    upsert(m.get, e);
    update(m.get, ID, { title: 'x', created: e.created, expires: 'later', published: true });
    expect(JSON.parse(m.value()!).workouts[0]).toEqual({ ...e, title: 'x', expires: 'later', published: true });
  });

  it('remove only with the matching token (P-12)', () => {
    const m = memory(list(entry({ token: OTHER })));
    remove(m.get, ID, TOKEN);
    expect(JSON.parse(m.value()!).workouts).toHaveLength(1);
    remove(m.get, ID, OTHER);
    expect(JSON.parse(m.value()!).workouts).toEqual([]);
  });

  it('never throws on failing storage', () => {
    const boom = () => {
      throw new Error('x');
    };
    const bad = { getItem: boom, setItem: boom, removeItem: boom };
    for (const s of [boom, () => bad, () => null]) {
      expect(() => upsert(s, e)).not.toThrow();
      expect(() => update(s, ID, e)).not.toThrow();
      expect(() => remove(s, ID, TOKEN)).not.toThrow();
      expect(forget(s, '{}')).toBe(false);
      expect(clear(s)).toBe(false);
    }
  });
});

describe('forget and clear (MW-4)', () => {
  it('forget removes exactly one entry, also one with a bad id', () => {
    const bad = entry({ id: 'javascript:x' });
    const m = memory(list(entry(), bad));
    const rows = prune(m.get, NOW)!;
    expect(forget(m.get, rows.find((r) => !r.edit)!.raw)).toBe(true);
    expect(JSON.parse(m.value()!).workouts).toEqual([entry()]);
  });

  it('clear removes the key', () => {
    const m = memory(list(entry()));
    expect(clear(m.get)).toBe(true);
    expect(m.value()).toBeUndefined();
  });
});
