// @vitest-environment jsdom
// The edit page, part 4: a split of the former edit.dom.test.ts so Vitest runs the parts in parallel; shared setup in testing/edit-dom.ts.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { flushSync } from 'svelte';
import { $, GENERIC, ID, ON, PUBLISHED_PAGE, RIDE, TOKEN, btn, click, expectNoTokenOutsideTheFragment, field, fresh, golden, mapReady, open, openPublished, patches, s, save, settle, shows, type } from './testing/edit-dom';
import { stubLog } from '../page/testing/stub-log';
import { decodeDataFile } from '../lib/datafile';
import { removableBoundaries, removeAutoLaps } from '../lib/addedlaps';
import { createRangeStats } from '../lib/range';
import { formatElapsed } from '../lib/format';
import { LIST_KEY } from '../lib/mine';

describe('F-65 AL-8, AL-9: added laps on the edit page', () => {
  const data = (() => {
    const r = decodeDataFile(RIDE);
    if (!r.ok) throw new Error('no ride');
    return r.data;
  })();
  const startS = Date.parse(data.meta.start_time) / 1000;
  const at = (i: number): string => new Date((startS + data.time[i]) * 1000).toISOString().slice(0, 19) + 'Z';
  const plus = (): HTMLButtonElement => document.querySelector<HTMLButtonElement>('.plus-button')!;
  const item = (text: string): HTMLButtonElement => [...document.querySelectorAll<HTMLButtonElement>('.plus-item')].find((b) => b.textContent === text)!;
  const reason = (text: string): string => (item(text).nextElementSibling as HTMLElement).textContent!;
  const rows = (): HTMLTableRowElement[] => [...document.querySelectorAll<HTMLTableRowElement>('section.laps .laps-scroll:not(.splits) tbody tr')];
  const lastCells = (): string[] => rows().map((r) => r.cells[r.cells.length - 2]?.textContent ?? '');
  async function start(): Promise<void> {
    await open();
    stubLog.chartStack[0].onready?.();
    await vi.waitFor(() => expect(document.querySelector('.pin-tools')).not.toBeNull());
    await settle();
    mapReady();
  }
  const pick = async (i: number): Promise<void> => {
    stubLog.chartStack[0].cursor.set(i);
    s.frames.splice(0).forEach((f) => f(0));
    await settle();
  };
  const addLap = async (): Promise<void> => {
    if (plus().getAttribute('aria-expanded') !== 'true') plus().click();
    item('Add lap').click();
    await settle();
  };

  it('"+": disabled without a cursor; opens a list with Add lap and Add pin; Esc, outside and "+" again close it', async () => {
    await start();
    expect(plus().disabled).toBe(true);
    await pick(1000);
    expect([plus().disabled, plus().getAttribute('aria-expanded'), plus().getAttribute('aria-label')]).toEqual([false, 'false', 'Add a lap or pin']);
    plus().click();
    const menu = document.querySelector<HTMLElement>('.plus-menu')!;
    expect([menu.hidden, plus().getAttribute('aria-expanded')]).toEqual([false, 'true']);
    expect([...menu.querySelectorAll('.plus-item')].map((b) => b.textContent)).toEqual(['Add lap', 'Add pin']);
    expect(plus().nextElementSibling).toBe(menu); // Tab reaches the items after "+"
    item('Add lap').focus();
    item('Add lap').dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    expect([menu.hidden, document.activeElement]).toEqual([true, plus()]);
    plus().click();
    document.body.dispatchEvent(new Event('pointerdown', { bubbles: true }));
    expect(menu.hidden).toBe(true);
    plus().click();
    plus().click();
    expect(menu.hidden).toBe(true);
    plus().click();
    stubLog.chartStack[0].cursor.set(null);
    s.frames.splice(0).forEach((f) => f(0));
    await settle();
    expect([menu.hidden, plus().disabled]).toEqual([true, true]);
  });

  it('AL-9: a boundary within 2 s of a device lap end counts as there already (cursor and both range ends)', async () => {
    await start();
    const e = data.meta.laps[0].end_index - 1;
    const near = data.time.findIndex((x, i) => i > e && x - data.time[e] > 0 && x - data.time[e] <= 2);
    const far = data.time.findIndex((x, i) => i > e && x - data.time[e] > 2);
    const sel = stubLog.chartStack[0].selection!;
    const range = async (a: number, b: number): Promise<void> => {
      sel.set(a, b);
      s.frames.splice(0).forEach((f) => f(0));
      await settle();
    };
    await pick(near);
    plus().click();
    expect([item('Add lap').getAttribute('aria-disabled'), reason('Add lap')]).toEqual(['true', 'Already a lap here']);
    await range(e, near); // both ends at or near the same end
    expect(item('Add lap').getAttribute('aria-disabled')).toBe('true');
    await range(near, far); // the far end is new: only it is added
    expect(item('Add lap').getAttribute('aria-disabled')).toBe('false');
    item('Add lap').click();
    await settle();
    expect(rows()).toHaveLength(17);
    await save();
    expect(patches()).toEqual([{ added_lap_boundaries: [at(far)] }]);
  });

  it('Add lap splits the lap at the cursor: one more row, "Added" then the device trigger, × only there; Save sends the set once', async () => {
    await start();
    expect(rows()).toHaveLength(16);
    await pick(1000);
    await addLap();
    expect(rows()).toHaveLength(17);
    const x = [...document.querySelectorAll<HTMLButtonElement>('.lap-x-btn')];
    expect(x.map((b) => b.getAttribute('aria-label'))).toEqual(['Remove the added lap boundary at the end of lap 2']);
    expect(rows()[1].querySelector('.lap-x-btn')).toBe(x[0]);
    expect(rows()[1].cells[0].textContent).toBe('2');
    expect(rows()[1].textContent).toContain('Added');
    expect(rows()[2].textContent).not.toContain('Added');
    expect(data.meta.laps).toHaveLength(16); // the test's own decode is untouched
    // On the boundary now: nothing to add, with the reason.
    plus().click();
    plus().click();
    expect([item('Add lap').getAttribute('aria-disabled'), reason('Add lap')]).toEqual(['true', 'Already a lap here']);
    expect(item('Add lap').getAttribute('aria-describedby')).toBe((item('Add lap').nextElementSibling as HTMLElement).id);
    await save();
    expect(patches()).toEqual([{ added_lap_boundaries: [at(1000)] }]);
    await save();
    expect(patches()).toHaveLength(1);
    // The × removes it, the row goes, and the next Save clears the set.
    x[0].click();
    await settle();
    expect(rows()).toHaveLength(16);
    expect(document.querySelector('.lap-x-btn')).toBeNull();
    await save();
    expect(patches().at(-1)).toEqual({ added_lap_boundaries: [] });
  });

  it('AL-8: the added-laps table under the pins: render, × in either table updates both, hidden when empty', async () => {
    await start();
    const tbl = (): HTMLElement => document.querySelector<HTMLElement>('.added-laps')!;
    const xs = (): HTMLButtonElement[] => [...tbl().querySelectorAll<HTMLButtonElement>('.added-x-btn')];
    expect(tbl().hidden).toBe(true);
    await pick(1000);
    await addLap();
    await pick(2000);
    await addLap();
    await vi.waitFor(() => expect(xs()).toHaveLength(2));
    expect(tbl().previousElementSibling).toBe(document.querySelector('.pin-list')); // directly under the pins
    expect(tbl().hidden).toBe(false);
    const lines = [...tbl().querySelectorAll('.pin-item:not(.added-head)')].map((l) => l.textContent);
    expect(lines[0]).toContain(formatElapsed(data.time[1000] - data.time[0]));
    expect(lines[0]!.startsWith('2')).toBe(true);
    expect(xs()[0].getAttribute('aria-label')).toBe('Remove the added lap boundary at the end of lap 2');
    // From the new table: both tables lose the row, focus goes to the next ×.
    xs()[0].click();
    await settle();
    expect(xs()).toHaveLength(1);
    expect(document.querySelectorAll('.lap-x-btn')).toHaveLength(1);
    expect(document.activeElement).toBe(xs()[0]);
    // From the main table: the new table follows and hides when empty.
    document.querySelector<HTMLButtonElement>('.lap-x-btn')!.click();
    await settle();
    expect(tbl().hidden).toBe(true);
    expect(rows()).toHaveLength(16);
  });

  it('with a range: both ends (OQ-304); an end on a device boundary adds nothing', async () => {
    await start();
    const sel = stubLog.chartStack[0].selection!;
    sel.set(1000, 1200);
    s.frames.splice(0).forEach((f) => f(0));
    await settle();
    await addLap();
    sel.set(1100, 1800); // 1800 is a device lap end
    s.frames.splice(0).forEach((f) => f(0));
    await settle();
    await addLap();
    await save();
    expect(patches()).toEqual([{ added_lap_boundaries: [at(1000), at(1100), at(1200)] }]);
    expect(rows()).toHaveLength(19);
  });

  it('a stored boundary shows at once; a trimmed one is refused, dimmed "Removed on Save" and not sent; an invalid stored one is dropped', async () => {
    s.settings = fresh({ added_lap_boundaries: ['2000-01-01T00:00:00Z', at(500), at(5000)] });
    await start();
    expect(rows()).toHaveLength(18);
    type('trim_a', '3000');
    await settle();
    expect(rows()[0].classList.contains('dim')).toBe(true);
    expect(rows()[0].querySelector('.lap-x')?.textContent).toBe('×Removed on Save');
    await pick(2000);
    plus().click();
    expect([item('Add lap').getAttribute('aria-disabled'), reason('Add lap')]).toEqual(['true', 'In the trimmed part']);
    await save();
    expect(patches()[0].added_lap_boundaries).toEqual([at(5000)]);
  });

  it('a lap boundary in a hidden start is allowed (no position check)', async () => {
    s.settings = fresh(ON);
    await start();
    await pick(20); // inside the 300 m start radius
    await addLap();
    await save();
    expect(patches()[0].added_lap_boundaries).toEqual([at(20)]);
  });

  describe('F-66 RA-8, RA-9: remove auto-laps', () => {
    const autoLine = (): HTMLButtonElement | null => document.querySelector<HTMLButtonElement>('.laps-auto-btn');
    const gone = removableBoundaries(data.meta, data.time, data.sampleCount, 0, data.sampleCount - 1);
    const after = removeAutoLaps(data, createRangeStats(data)).length;

    it('the line shows the count, takes effect at once, Save sends the setting, Restore brings the device laps back', async () => {
      expect(gone).toBeGreaterThan(0);
      await start();
      expect(autoLine()!.textContent).toBe(`Remove auto-laps (${gone})`);
      autoLine()!.click();
      await settle();
      expect(rows()).toHaveLength(after);
      expect(autoLine()!.textContent).toBe('Restore auto-laps');
      await save();
      expect(patches()).toEqual([{ remove_auto_laps: true }]);
      autoLine()!.click();
      await settle();
      expect(rows()).toHaveLength(16);
      expect(autoLine()!.textContent).toBe(`Remove auto-laps (${gone})`);
    });

    it('the count follows the live trim window: only boundaries strictly inside it', async () => {
      await start();
      const hi = data.meta.laps[4].end_index - 1;
      const want = removableBoundaries(data.meta, data.time, data.sampleCount, 0, hi);
      expect(want).toBeLessThan(gone);
      type('trim_b', String(hi));
      await settle();
      expect(autoLine()?.textContent ?? null).toBe(want === 0 ? null : `Remove auto-laps (${want})`);
    });

    it('a stored on setting shows the removed laps and "Restore"; an added boundary on a restored device boundary is left out of the next Save', async () => {
      const merged = new Set(removeAutoLaps(data, createRangeStats(data)).map((l) => l.end_index - 1));
      const removedEnd = data.meta.laps.map((l) => l.end_index - 1).find((k) => !merged.has(k) && k > 0 && k < data.sampleCount - 1)!;
      expect(removedEnd).toBeGreaterThan(0);
      s.settings = fresh({ remove_auto_laps: true, added_lap_boundaries: [at(removedEnd)] });
      await start();
      expect(autoLine()!.textContent).toBe('Restore auto-laps');
      expect(rows()).toHaveLength(after === 0 ? 2 : after + 1); // the added boundary splits a merged lap, or the whole workout
      autoLine()!.click();
      await settle();
      await save();
      expect(patches()).toEqual([{ added_lap_boundaries: [], remove_auto_laps: false }]);
    });
  });

  describe('F-66 QA-66-r1-1: no distance, no lengths', () => {
    const line = (): HTMLButtonElement | null => document.querySelector<HTMLButtonElement>('.laps-auto-btn');
    /** The golden's one implicit lap replaced by a time lap and a stop lap (metadata rewritten, columns shifted). */
    const plain = (): void => {
      const src = golden('indoor-no-distance-no-gps');
      const v = new DataView(src);
      const l = v.getUint32(10, true);
      const meta = JSON.parse(new TextDecoder().decode(new Uint8Array(src, 14, l)));
      const n = v.getUint32(6, true);
      const mid = Math.floor(n / 2);
      const lap = (a: number, b: number, trigger: string) => ({ start: meta.laps[0].start, end: meta.laps[0].end, start_index: a, end_index: b, trigger, stroke: '', totals: {} });
      meta.laps = [lap(0, mid, 'time'), lap(mid, n, 'stop')];
      const a4 = (x: number): number => Math.ceil(x / 4) * 4;
      const d = a4(14 + new TextEncoder().encode(JSON.stringify(meta)).length) - a4(14 + l);
      for (const st of meta.streams) st.byte_offset += d;
      const out = new TextEncoder().encode(JSON.stringify(meta));
      const head = a4(14 + out.length);
      const buf = new Uint8Array(head + src.byteLength - a4(14 + l));
      buf.set(new Uint8Array(src, 0, 14));
      new DataView(buf.buffer).setUint32(10, out.length, true);
      buf.set(out, 14);
      buf.set(new Uint8Array(src, a4(14 + l)), head);
      s.draft = () => new Response(buf.buffer.slice(0), { status: 200 });
    };

    it('removing every auto-lap keeps "Restore auto-laps"; it brings the laps back', async () => {
      plain();
      await start();
      expect(line()!.textContent).toMatch(/^Remove auto-laps \(\d+\)$/);
      line()!.click();
      await settle();
      expect(line()!.textContent).toBe('Restore auto-laps');
      line()!.click();
      await settle();
      expect(line()!.textContent).toMatch(/^Remove auto-laps/);
    });

    it('opening with the setting stored on shows "Restore auto-laps"', async () => {
      plain();
      s.settings = fresh({ remove_auto_laps: true });
      await start();
      expect(line()!.textContent).toBe('Restore auto-laps');
    });
  });

  it('at 100 added boundaries Add lap is off with its reason', async () => {
    s.settings = fresh({ added_lap_boundaries: Array.from({ length: 100 }, (_, k) => at(11 + 10 * k)) });
    await start();
    await pick(5003);
    plus().click();
    expect([item('Add lap').getAttribute('aria-disabled'), reason('Add lap')]).toEqual(['true', 'Max. 100 added laps']);
  });

  it('without device laps: the first boundary brings the laps view, removing it returns to splits only', async () => {
    const plain = golden('ride-no-laps');
    s.draft = () => new Response(plain.slice(0), { status: 200 });
    await start();
    const section = document.querySelector<HTMLElement>('section.laps')!;
    expect(section.querySelector('.laps-scroll:not(.splits)')).toBeNull();
    await pick(300);
    await addLap();
    await vi.waitFor(() => expect(rows()).toHaveLength(2));
    expect(lastCells()).toEqual(['Added', '']);
    await vi.waitFor(() => expect(document.querySelector<HTMLElement>('section.laps')?.hidden).toBe(false));
    document.querySelector<HTMLButtonElement>('.lap-x-btn')!.click();
    await settle();
    expect(rows()).toHaveLength(0);
  });

  it('QA-65-r1-2: the live trim end is a boundary; focus leaving the group closes the list; a choice returns focus to "+"', async () => {
    await start();
    type('trim_b', '5000');
    await settle();
    await pick(5000);
    plus().click();
    expect([item('Add lap').getAttribute('aria-disabled'), reason('Add lap')]).toEqual(['true', 'Already a lap here']);
    const menu = document.querySelector<HTMLElement>('.plus-menu')!;
    item('Add lap').dispatchEvent(new FocusEvent('focusout', { bubbles: true, relatedTarget: field('title') }));
    expect(menu.hidden).toBe(true);
    await pick(1000);
    plus().click();
    item('Add lap').focus();
    item('Add lap').click();
    expect([menu.hidden, document.activeElement]).toEqual([true, plus()]);
  });

  it('QA-65-r1-2: after a change the map lap markers, the splits chunk (its switch) and the row taps start again', async () => {
    await start();
    const laps = vi.fn();
    const handle = { setScheme() {}, setHidden() {}, marker: () => () => {}, marks() {}, laps, control() {}, sampleAt: () => -1, destroy() {} };
    (stubLog.routeMap[0] as unknown as { onmapready(h: typeof handle): void }).onmapready(handle);
    await settle();
    await pick(1000);
    await addLap();
    await vi.waitFor(() => expect(laps).toHaveBeenCalledTimes(1));
    const section = document.querySelector<HTMLElement>('section.laps')!;
    await vi.waitFor(() => expect(section.querySelector('#laps-slot')!.childElementCount).toBeGreaterThan(0)); // the switch
    expect(section.classList.contains('live')).toBe(true);
    rows()[1].click(); // the new first part: samples 900 to 1000
    s.frames.splice(0).forEach((f) => f(0));
    const sel = stubLog.chartStack[0].selection!;
    expect([sel.start(), sel.end()]).toEqual([900, 1000]);
    // ×: the module is rebuilt and focus lands on the laps heading (no other ×).
    document.querySelector<HTMLButtonElement>('.lap-x-btn')!.click();
    await settle();
    expect(laps).toHaveBeenCalledTimes(2);
    expect(document.activeElement).toBe(document.querySelector('section.laps h2'));
  });

  it('QA-65-r1-5: boundaries the live trim removes do not count towards the 100', async () => {
    s.settings = fresh({ added_lap_boundaries: Array.from({ length: 100 }, (_, k) => at(11 + 10 * k)) });
    await start();
    type('trim_a', '500');
    await settle();
    await pick(5003);
    plus().click();
    expect(item('Add lap').getAttribute('aria-disabled')).toBe('false');
  });

  it('QA-65-r1-3: on a multisport workout the × names the lap number its leg table shows', async () => {
    s.draft = () => new Response(golden('triathlon').slice(0), { status: 200 });
    await start();
    const tri = decodeDataFile(golden('triathlon'));
    if (!tri.ok) throw new Error('no triathlon');
    const bike = tri.data.meta.sessions.find((x) => x.sport === 'cycling')!;
    const firstBikeLap = tri.data.meta.laps.find((l) => l.start_index >= bike.start_index)!;
    await pick(firstBikeLap.start_index + 10);
    await addLap();
    const x = document.querySelector<HTMLButtonElement>('.lap-x-btn')!;
    expect(x.closest('tr')!.cells[0].textContent).toBe('1');
    expect(x.getAttribute('aria-label')).toBe('Remove the added lap boundary at the end of lap 1');
  });

  it('no visible route (OQ-305): "+" sits below the no-location hint and offers only Add lap', async () => {
    const indoor = golden('indoor-no-gps');
    s.draft = () => new Response(indoor.slice(0), { status: 200 });
    s.settings = fresh({ hiding_steps: null });
    await start();
    const hint = document.querySelector('p.hint')!;
    expect(hint.compareDocumentPosition(plus()) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    await pick(100);
    plus().click();
    expect([...document.querySelectorAll<HTMLElement>('.plus-row')].map((r) => r.hidden)).toEqual([false]); // no Add pin item at all (no positions)
  });
});

describe('F-53 MW-2, MW-3: the My workouts entry (AC 42, 44, 46, 48, 49, 51, 52)', () => {
  const OTHER = 'Yy0123456789abcdefghij';
  const stored = () => JSON.parse(localStorage.getItem(LIST_KEY) ?? 'null') as { v: number; workouts: Record<string, unknown>[] } | null;
  const entry = () => stored()?.workouts.find((e) => e.id === ID);
  const meta = () => {
    const d = decodeDataFile(RIDE.slice(0));
    if (!d.ok) throw new Error('golden');
    return d.data.meta;
  };
  const put = (...workouts: Record<string, unknown>[]) => localStorage.setItem(LIST_KEY, JSON.stringify({ v: 1, workouts }));

  it('the first read adds one entry with the header sport and local date, Draft, the stored instants; nothing sent', async () => {
    s.settings = fresh({ suggested_title: 'Morning ride' });
    await open();
    const m = meta();
    const date = new Date(Date.parse(m.start_time) + (m.time_zone.known ? m.time_zone.offset_seconds : 0) * 1000).toISOString().slice(0, 10);
    expect(stored()).toEqual({
      v: 1,
      workouts: [{ id: ID, token: TOKEN, title: 'Morning ride', sport: m.sport, date, created: s.settings.created_at, expires: s.settings.expires_at, published: false }],
    });
    for (const { url, init } of s.calls) expect(url + String(init.body ?? '')).not.toMatch(/Morning ride|wx\.workouts/);
    expect(document.cookie).toBe('');
    expectNoTokenOutsideTheFragment();
  });

  it('the default title applies: title stored empty; an owner title wins', async () => {
    await open();
    expect(entry()?.title).toBe('');
    type('title', 'Loop');
    await save();
    expect(entry()?.title).toBe('Loop');
  });

  it('one entry per workout: reopening and saving replace it, others stay; an unreadable value is replaced', async () => {
    put({ id: 'B'.repeat(22), token: OTHER, title: 'Other' }, { id: ID, token: TOKEN, title: 'old', sport: 'rowing', date: '2020-01-01' });
    await open();
    expect(entry()).toMatchObject({ title: '', sport: meta().sport });
    type('title', 'New');
    await save();
    expect(stored()!.workouts.map((e) => e.id)).toEqual(['B'.repeat(22), ID]);
    expect(entry()).toMatchObject({ title: 'New', sport: meta().sport, expires: s.settings.expires_at });
    localStorage.setItem(LIST_KEY, '{"v":2}');
    await open();
    expect(stored()!.workouts.map((e) => e.id)).toEqual([ID]);
    put({ id: ID, token: OTHER, title: 'old' });
    await open();
    expect(entry()?.token).toBe(TOKEN); // QA-53-r1-3: the address's token wins
  });

  it('the draft unreadable: the stored sport and date stay', async () => {
    put({ id: ID, token: TOKEN, title: 'x', sport: 'rowing', date: '2020-01-01' });
    s.draft = () => new Response(new Uint8Array([1, 2, 3]), { status: 200 });
    await open();
    expect(entry()).toMatchObject({ sport: 'rowing', date: '2020-01-01', title: '' });
  });

  it('publish sets Published before the navigation; the published page updates title, status, expiry of an entry and never adds one', async () => {
    await open();
    await click('publish');
    expect(entry()?.published).toBe(true);
    localStorage.clear();
    s.settings = fresh({ visibility: 'unlisted', title: 'Pub', expires_at: '2026-11-05T11:00:00Z' });
    await openPublished();
    expect(stored()).toBeNull();
    put({ id: ID, token: TOKEN, title: 'old', sport: 'rowing', published: false });
    await openPublished();
    expect(entry()).toEqual({ id: ID, token: TOKEN, title: 'Pub', sport: 'rowing', created: s.settings.created_at, expires: '2026-11-05T11:00:00Z', published: true });
    expect(document.body.textContent).toContain('Edit links are saved in this browser. Anyone using this browser can open them.');
  });

  it.each([
    ['edit', () => open()],
    ['published', () => openPublished()],
  ])('delete on the %s page removes the entry', async (_, go) => {
    s.settings = fresh({ visibility: 'unlisted' });
    put({ id: ID, token: TOKEN, title: 'x' }, { id: 'B'.repeat(22), token: OTHER });
    await go();
    expect(entry()).toBeDefined();
    await click('delete');
    await click('confirm');
    expect(stored()!.workouts.map((e) => e.id)).toEqual(['B'.repeat(22)]);
  });

  it('P-12: not found removes the entry only when it holds the address token', async () => {
    s.onGet = () => new Response('{}', { status: 404 });
    put({ id: ID, token: OTHER, title: 'mine' });
    await open();
    expect(document.body.textContent).toContain(GENERIC);
    expect(entry()?.token).toBe(OTHER);
    put({ id: ID, token: TOKEN, title: 'mine' });
    await open();
    expect(stored()!.workouts).toEqual([]);
    put({ id: ID, token: TOKEN, title: 'mine' });
    await openPublished();
    expect(stored()!.workouts).toEqual([]);
  });

  it('not found on save also removes it', async () => {
    await open();
    s.onPatch = () => new Response('', { status: 404 });
    type('title', 'x');
    await save();
    expect(stored()!.workouts).toEqual([]);
  });

  it.each([
    ['a throwing getter', () => vi.spyOn(window, 'localStorage', 'get').mockImplementation(() => { throw new DOMException('blocked', 'SecurityError'); })],
    ['a full storage', () => vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new DOMException('full', 'QuotaExceededError'); })],
    ['a throwing getItem', () => vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new DOMException('blocked', 'SecurityError'); })],
  ])('AC 52, %s: edit, save, publish and delete still work, no message', async (_, block) => {
    block();
    await open();
    expect($('form.edit-panel')).not.toBeNull();
    type('title', 'x');
    await save();
    expect($('.edit-status')?.textContent).toBe('Saved');
    await click('publish');
    expect(s.navs).toEqual([PUBLISHED_PAGE()]);
    expect(s.consoleOut).toEqual([]);
  });
});

