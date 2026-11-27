// @vitest-environment jsdom
// The edit page, part 3: a split of the former edit.dom.test.ts so Vitest runs the parts in parallel; shared setup in testing/edit-dom.ts.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { flushSync } from 'svelte';
import { $, FIT, N, ON, RIDE, STEPS, btn, controlPos, field, fresh, golden, hideAt, mapReady, open, patches, s, save, settle, shows, type } from './testing/edit-dom';
import { stubLog } from '../page/testing/stub-log';
import type { Settings } from '../lib/owner';
import { decodeDataFile } from '../lib/datafile';
import { en as editTexts } from './texts';

describe('F-49 ZN-4, ZN-7: zone fields and the Zones section (AC 6, 7, 9)', () => {
  const decoded = decodeDataFile(RIDE.slice(0));
  if (!decoded.ok) throw new Error('ride-4h does not decode');
  const T = decoded.data.time;
  const S = Date.parse(decoded.data.meta.start_time) / 1000;
  const at = (i: number): string => new Date((S + T[i]) * 1000).toISOString().slice(0, 19) + 'Z';
  const zones = (): HTMLElement => $('#zones-slot')!;
  const bounds = (kind: string): string[] => [...zones().querySelectorAll(`.zone[data-k="${kind}"] .zone-r`)].map((e) => e.textContent!);
  const seconds = (kind: string): number =>
    [...zones().querySelectorAll(`.zone[data-k="${kind}"] .zone-t`)].map((e) => e.textContent!.split(':').reduce((s, p) => s * 60 + Number(p), 0)).reduce((x, y) => x + y, 0);
  const paint = async (): Promise<void> => {
    stubLog.chartStack[0].onready?.();
    await settle();
  };

  it('a file without zone values: no section until a value is saved; then the sharer\'s zones; cleared, none again', async () => {
    await open();
    await paint();
    expect([field('zones_max_heart_rate').value, field('zones_threshold_power').value]).toEqual(['', '']);
    expect(zones().hidden).toBe(true);
    type('zones_threshold_power', '250');
    await settle();
    expect(zones().hidden).toBe(true); // unsaved input changes nothing (E-6)
    await save();
    expect(patches()).toEqual([{ zones_threshold_power: 250 }]);
    await vi.waitFor(() => expect(bounds('power')).toHaveLength(7));
    expect(bounds('power')[5]).toBe('138–187 W'); // 137.5 → 138, 187.5 → 188; Z2, highest on top
    expect([...zones().querySelectorAll('.zones-src')].map((e) => e.textContent)).toEqual(['Estimated from threshold power 250 W']);
    expect(field('zones_threshold_power').value).toBe('250');
    type('zones_threshold_power', '');
    await save();
    expect(patches()[1]).toEqual({ zones_threshold_power: null });
    expect(zones().hidden).toBe(true);
  });

  it('a stored value fills its field and is the basis from the start', async () => {
    s.settings = fresh({ zones_max_heart_rate: 190 });
    await open();
    await paint();
    await vi.waitFor(() => expect(bounds('heart_rate')).toHaveLength(5));
    expect(field('zones_max_heart_rate').value).toBe('190');
    expect(bounds('heart_rate')[4]).toBe('≤ 113 bpm'); // highest zone on top
    expect([...zones().querySelectorAll('.zones-src')].map((e) => e.textContent)).toEqual(['Estimated from max heart rate 190 bpm']);
  });

  it('ZN-7, ZN-10, AC 2b: the file basis is the placeholder; empty keeps the device zones, a typed value (even the same) is the sharer\'s', async () => {
    s.draft = () => new Response(golden('zones-wahoo-ride').slice(0), { status: 200 });
    const captions = (): string[] => [...zones().querySelectorAll('.zones-src')].map((e) => e.textContent!);
    await open();
    await paint();
    // Power has a basis next to its list; heart rate only a list: no placeholder.
    expect([field('zones_max_heart_rate').placeholder, field('zones_threshold_power').placeholder]).toEqual(['', '250']);
    expect([field('zones_max_heart_rate').value, field('zones_threshold_power').value]).toEqual(['', '']);
    expect([...document.querySelectorAll('.edit-small')].map((e) => e.textContent)).toContain("For the heart-rate and power zones. Empty: the device's zones and values from the FIT file are used.");
    await vi.waitFor(() => expect(bounds('power')).toHaveLength(7));
    expect(captions()).toEqual(['Zones as recorded by the device', 'Zones as recorded by the device (threshold power 250 W)']);
    expect(bounds('power')[5]).toBe('139–188 W'); // the device's list
    type('title', 'x');
    await save();
    expect(patches()).toEqual([{ title: 'x' }]); // empty fields send nothing new: null stays null
    type('zones_threshold_power', '250');
    await save();
    expect(patches()[1]).toEqual({ zones_threshold_power: 250 });
    await vi.waitFor(() => expect(captions()[1]).toBe('Estimated from threshold power 250 W'));
    expect(bounds('power')[5]).toBe('138–187 W'); // the model replaced the whole kind
    type('zones_threshold_power', '');
    await save();
    expect(patches()[2]).toEqual({ zones_threshold_power: null });
    await vi.waitFor(() => expect(captions()[1]).toBe('Zones as recorded by the device (threshold power 250 W)'));
    expect(field('zones_threshold_power').placeholder).toBe('250');
  });

  it('OQ-195: a value the server refuses is not sent; its sentence shows until the next edit', async () => {
    await open();
    type('zones_max_heart_rate', '99');
    await save();
    expect(patches()).toEqual([]);
    expect($('.edit-status')?.textContent).toBe('The max heart rate must be a whole number from 100 to 250 bpm.');
    type('zones_max_heart_rate', '250');
    expect($('.edit-status')?.textContent).toBe('');
    type('zones_threshold_power', '1000.5');
    $<HTMLButtonElement>('[data-act="publish"]')!.click();
    await settle();
    expect(s.calls.filter((c) => c.url.endsWith('/publish'))).toEqual([]);
    expect($('.edit-status')?.textContent).toBe('The threshold power must be a whole number from 50 to 1,000 W.');
  });

  it('TR-9, AC 9: the zones count only the saved trim window; an unsaved trim changes nothing', async () => {
    s.settings = fresh({ zones_threshold_power: 250, trim_start_at: at(1000), trim_end_at: at(1600) });
    await open();
    await paint();
    await vi.waitFor(() => expect(bounds('power')).toHaveLength(7));
    const saved = seconds('power');
    expect(saved).toBeGreaterThan(0);
    expect(saved).toBeLessThanOrEqual(600);
    type('trim_b', String(N - 1));
    await settle();
    expect(seconds('power')).toBe(saved);
  });
});

describe('F-50 EF-1: the Best efforts section over the saved trim window (AC 5)', () => {
  const decoded = decodeDataFile(RIDE.slice(0));
  if (!decoded.ok) throw new Error('ride-4h does not decode');
  const T = decoded.data.time;
  const S = Date.parse(decoded.data.meta.start_time) / 1000;
  const at = (i: number): string => new Date((S + T[i]) * 1000).toISOString().slice(0, 19) + 'Z';
  const names = (): string[] => [...document.querySelectorAll('#efforts-slot tbody td:first-child')].map((e) => e.textContent!);

  it('only the saved window counts; an unsaved trim changes nothing; a save that changes it recomputes', async () => {
    s.settings = fresh({ trim_start_at: at(1000), trim_end_at: at(1600) });
    await open();
    stubLog.chartStack[0].onready?.();
    await vi.waitFor(() => expect(names()).toEqual(['5 s', '1 min', '5 min']));
    const table = $('#efforts-slot table');
    type('title', 'x');
    await save();
    expect($('#efforts-slot table')).toBe(table); // same window: no recompute
    type('trim_b', String(N - 1));
    await settle();
    expect($('#efforts-slot table')).toBe(table);
    await save();
    await vi.waitFor(() => expect(names()).toEqual(['5 s', '1 min', '5 min', '20 min', '60 min']));
  });
});

describe('F-58 SE-4: the summary dialog on the edit page (AC 36, 40)', () => {
  const decoded = decodeDataFile(RIDE.slice(0));
  if (!decoded.ok) throw new Error('ride-4h does not decode');
  const { calories, ...rest } = decoded.data.meta.summary;
  const dialogText = async (): Promise<string> => {
    $<HTMLButtonElement>('button[data-summary-info]')!.click();
    await vi.waitFor(() => expect($<HTMLDialogElement>('dialog.summary-dlg')?.open).toBe(true));
    const d = $<HTMLDialogElement>('dialog.summary-dlg')!;
    d.close();
    return d.textContent!;
  };
  beforeEach(() => {
    HTMLDialogElement.prototype.showModal = function (this: HTMLDialogElement) {
      this.open = true;
    };
    HTMLDialogElement.prototype.close = function (this: HTMLDialogElement) {
      this.open = false;
    };
    localStorage.setItem('wx.units', 'metric');
  });

  it('radii, trim ends and the recomputed and dropped totals from the saved settings; rebuilt on every open', async () => {
    expect(calories).toBeGreaterThan(0);
    s.settings = fresh({ ...ON, trim_start_at: '2026-10-29T10:00:00Z', trim_end_at: '2026-10-29T11:00:00Z', trimmed: { sport: decoded.data.meta.sport, summary: rest } });
    await open();
    const button = $<HTMLButtonElement>('button[data-summary-info]')!;
    expect([button.classList.contains('ready'), button.dataset.trim]).toEqual([true, 'both']);
    let text = await dialogText();
    expect(text).toContain('This workout: start hidden within 300 m, end hidden within 300 m.');
    expect(text).toMatch(/This workout: trimmed \(now starts \d\d:\d\d, ends \d\d:\d\d\); recomputed: distance, [^;]*; dropped: calories\./);
    hideAt(0, 0);
    text = await dialogText();
    expect(text).toContain('This workout: start hidden within 300 m, end hidden within 300 m.'); // the saved radii, not the sliders
    await save();
    text = await dialogText();
    expect(text).toContain('This workout: start not hidden, end not hidden.');
  });

  it('names the trimmed end, and a trim save refreshes it without a reload (QA-58-r1-5)', async () => {
    s.settings = fresh({ trim_start_at: '2026-10-29T10:00:00Z' });
    await open();
    const button = $<HTMLButtonElement>('button[data-summary-info]')!;
    expect(button.dataset.trim).toBe('start');
    let text = await dialogText();
    expect(text).toMatch(/trimmed \(now starts \d\d:\d\d\)/);
    type('trim_a', '0');
    type('trim_b', String(N - 200));
    await save();
    expect(button.dataset.trim).toBe('end');
    text = await dialogText();
    expect(text).toMatch(/trimmed \(now ends \d\d:\d\d\)/);
    expect(text).not.toContain('starts');
  });

  it('untrimmed: no trim line, data-trim empty', async () => {
    await open();
    expect($<HTMLButtonElement>('button[data-summary-info]')!.dataset.trim).toBe('');
    expect(await dialogText()).not.toMatch(/Trim\.|recomputed/);
  });
});

describe('F-56 PN-12: pins on the edit page (AC 33 to 36, 41)', () => {
  const data = (() => {
    const r = decodeDataFile(RIDE);
    if (!r.ok) throw new Error('no ride');
    return r.data;
  })();
  const startS = Date.parse(data.meta.start_time) / 1000;
  const at = (i: number): string => new Date((startS + data.time[i]) * 1000).toISOString().slice(0, 19) + 'Z';
  const pin = (i: number, title: string) => ({ type: 'danger' as const, title, note: '', start_at: at(i), end_at: null });
  const rowTexts = (): string[] => [...document.querySelectorAll('.pin-row')].map((r) => r.textContent!);
  const button = (text: string): HTMLButtonElement => [...document.querySelectorAll<HTMLButtonElement>('.pin-tools button, .wx-addpin button, dialog.pin-dlg button')].find((b) => b.textContent === text)!;
  /** F-65 AL-9: "+" opened (when it can be), then the item's disabled state; "+" disabled counts as off. */
  const plusButton = (): HTMLButtonElement => document.querySelector<HTMLButtonElement>('.plus-button')!;
  const off = (text: string): boolean => {
    const plus = plusButton();
    if (plus.disabled) return true;
    if (plus.getAttribute('aria-expanded') !== 'true') plus.click();
    return button(text).getAttribute('aria-disabled') === 'true';
  };
  // jsdom has no modal dialogs: the open state and the close event are all the editor relies on.
  beforeEach(() => {
    HTMLDialogElement.prototype.showModal = function (this: HTMLDialogElement) {
      this.open = true;
    };
    HTMLDialogElement.prototype.close = function (this: HTMLDialogElement) {
      if (!this.open) return;
      this.open = false;
      this.dispatchEvent(new Event('close'));
    };
  });
  const dlg = (): HTMLDialogElement => document.querySelector<HTMLDialogElement>('dialog.pin-dlg')!;
  const dlgOpen = async (): Promise<HTMLDialogElement> => {
    await vi.waitFor(() => expect(dlg()?.open).toBe(true));
    return dlg();
  };
  const field = (name: 'select' | 'input' | 'textarea') => dlg().querySelector<HTMLInputElement>(name)!;
  const fill = (el: HTMLInputElement, v: string): void => {
    el.value = v;
    el.dispatchEvent(new Event('input', { bubbles: true }));
  };
  const submit = (): void => void dlg().querySelector<HTMLFormElement>('form')!.dispatchEvent(new Event('submit', { cancelable: true }));
  const dlgButton = (text: string): HTMLButtonElement => [...dlg().querySelectorAll('button')].find((b) => b.textContent === text)!;
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

  it('dimmed reasons; Add pin refuses a hidden sample, places on a visible one; Save sends the whole set, dimmed included', async () => {
    s.settings = fresh({ ...ON, pins: [pin(10, 'start'), pin(1000, 'mid'), pin(N - 5, 'end')] });
    await start();
    expect(rowTexts()).toEqual(['startHidden by the start radius', expect.stringMatching(/^mid\d/), 'endHidden by the end radius']);
    expect(off('Add pin')).toBe(true); // no cursor
    await pick(20); // in the hidden start radius: refused, nothing placed
    expect(off('Add pin')).toBe(true);
    button('Add pin').click();
    expect(dlg()).toBeNull();
    expect(rowTexts()).toHaveLength(3);
    await pick(2000);
    expect(off('Add pin')).toBe(false);
    button('Add pin').click();
    await dlgOpen();
    expect(rowTexts()).toHaveLength(3); // nothing until OK
    fill(field('input'), '<b>new</b>');
    submit();
    expect(dlg().open).toBe(false);
    expect(rowTexts()).toHaveLength(4);
    expect(rowTexts()[2]).toMatch(/^<b>new<\/b>/);
    expect(document.querySelector('.pin-list b')).toBeNull();
    await save();
    expect(patches()).toEqual([{ pins: [pin(10, 'start'), pin(1000, 'mid'), { ...pin(2000, '<b>new</b>'), type: 'point_of_interest' }, pin(N - 5, 'end')] }]);
    await save();
    expect(patches()).toHaveLength(1); // unchanged: no pins sent, 'No changes'
  });

  it('a Save whose response lacks pins says how many were removed (AC 36)', async () => {
    s.settings = fresh({ pins: [pin(10, 'a'), pin(1000, 'b')] });
    s.onPatch = (body) => Response.json((s.settings = { ...s.settings, ...body, pins: [pin(1000, 'b')] } as Settings));
    await start();
    $<HTMLInputElement>('input[name="title"]')!.value = 'x';
    await save();
    expect(document.querySelector('.pin-tools .edit-status')?.textContent).toBe('1 pin in the trimmed part removed.');
    expect(rowTexts()).toHaveLength(1);
  });

  it('QA-56d-r1-1: deleting pins with added laps present keeps focus on a pin row, then the heading', async () => {
    s.settings = fresh({ ...ON, pins: [pin(1000, 'a'), pin(1500, 'b')], added_lap_boundaries: [at(2500)] });
    await start();
    await vi.waitFor(() => expect(document.querySelector('.added-laps:not([hidden])')).not.toBeNull());
    const x = (): HTMLButtonElement[] => [...document.querySelectorAll<HTMLButtonElement>('.pin-item[data-pin] .added-x-btn')];
    x()[1].click();
    expect(document.activeElement).toBe(row0());
    x()[0].click();
    expect(document.activeElement).toBe(document.querySelector('.pins-head'));
  });

  it('QA-56d-r1-2: a Save that replaces the list while the dialog is open: Delete and OK touch no other pin', async () => {
    s.settings = fresh({ ...ON, pins: [pin(1000, 'a'), pin(1500, 'b')] });
    await start();
    await open0();
    s.settings = fresh({ ...ON, pins: [pin(1500, 'b')] }); // the server dropped a
    fill($<HTMLInputElement>('input[name="title"]')!, 'x');
    await save();
    dlgButton('Delete pin').click();
    expect(rowTexts()).toHaveLength(1);
    expect(rowTexts()[0]).toMatch(/^b/);
    await open0();
    fill(field('input'), 'changed');
    s.settings = fresh({ ...ON, pins: [pin(1500, 'b')] });
    fill($<HTMLInputElement>('input[name="title"]')!, 'y');
    await save();
    submit();
    expect(rowTexts()[0]).toMatch(/^b/);
  });

  it('QA-56d-r1-3: a failed dialog import resets the editor so the next open works', async () => {
    s.settings = fresh({ ...ON, pins: [pin(1000, 'a')] });
    await start();
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});
    vi.doMock('../dialog', () => {
      throw new Error('chunk gone');
    });
    row0().click();
    await vi.waitFor(() => expect(warn).toHaveBeenCalled());
    vi.doUnmock('../dialog');
    expect(document.querySelector('.pin-item.sel')).toBeNull();
    await open0();
    expect(dlg().open).toBe(true);
    warn.mockRestore();
  });

  it('AC 4: a dimmed pin opens in the dialog with its reason and can be deleted', async () => {
    s.settings = fresh({ ...ON, pins: [pin(10, 'start'), pin(1000, 'mid')] });
    await start();
    await open0();
    expect(dlg().textContent).toContain('Hidden by the start radius');
    dlgButton('Delete pin').click();
    expect(rowTexts()).toHaveLength(1);
    expect(rowTexts()[0]).toMatch(/^mid/);
  });

  it('no visible route: no Add pin, stored pins listed dimmed (OQ-275)', async () => {
    s.settings = fresh({ hide_start_m: 2000, hiding_steps: { start: STEPS.start.map(() => -1), end: STEPS.end }, pins: [pin(1000, 'b')] });
    await start();
    await pick(1000);
    plusButton().click();
    expect((button('Add pin').parentElement as HTMLElement).hidden).toBe(true);
    expect((button('Add lap').parentElement as HTMLElement).hidden).toBe(false); // OQ-305: only Add lap
    expect(rowTexts()[0]).toMatch(/Hidden by the start/);
  });

  it('Add pin over a range makes a section once OK; a range with a hidden end offers no Add pin (AC 34, PIN-8)', async () => {
    s.settings = fresh({ ...ON, pins: [] });
    await start();
    const sel = stubLog.chartStack[0].selection!;
    const range = async (a: number, b: number): Promise<void> => {
      sel.set(a, b);
      s.frames.splice(0).forEach((f) => f(0));
      await settle();
    };
    await range(2000, 3000);
    button('Add pin').click();
    await dlgOpen();
    expect(dlg().querySelector('.edit-small')!.textContent).toMatch(/^[\d.]+–[\d.]+ (mi|km)$/);
    submit();
    expect(rowTexts()[0]).toMatch(/–/);
    await range(2600, N - 5); // the end is in the hidden end radius
    expect(off('Add pin')).toBe(true);
    await save();
    expect(patches()).toEqual([{ pins: [{ ...pin(2000, ''), type: 'point_of_interest', end_at: at(3000) }] }]);
  });

  it('PN-12, AL-9: in the two-column layout "+" with Add pin and its reason docks on the map slot, one column keeps them in the tools row', async () => {
    let matches = true;
    const listeners: ((e: { matches: boolean }) => void)[] = [];
    vi.stubGlobal('matchMedia', () => ({ get matches() { return matches; }, addEventListener: (_: string, f: (e: { matches: boolean }) => void) => listeners.push(f), removeEventListener() {} }));
    s.settings = fresh({ ...ON, pins: [] });
    await start();
    const add = plusButton();
    expect(add.closest('#map-slot .wx-addpin')).not.toBeNull();
    expect(controlPos).toContain('top-right'); // the right-hand column, below the other map buttons (F-65 AL-9)
    expect(button('Add pin').closest('.wx-plus')).toBe(add.closest('.wx-plus'));
    matches = false;
    listeners.forEach((f) => f({ matches }));
    expect(add.closest('.pin-tools')).not.toBeNull();
    expect(document.querySelector('.pin-tools')?.classList.contains('on-map')).toBe(false);
    // one column: no "+", both items open side by side; back in two columns the "+" returns closed
    expect([add.hidden, document.querySelector<HTMLElement>('.plus-menu')!.hidden]).toEqual([true, false]);
    expect([...document.querySelectorAll<HTMLElement>('.plus-item')].map((b) => [b.textContent, b.getAttribute('aria-label')])).toEqual([['+ Lap', 'Add lap'], ['+ Pin', 'Add pin']]);
    matches = true;
    listeners.forEach((f) => f({ matches }));
    expect([add.hidden, add.getAttribute('aria-expanded'), document.querySelector<HTMLElement>('.plus-menu')!.hidden]).toEqual([false, 'false', true]);
    expect([...document.querySelectorAll<HTMLElement>('.plus-item')].map((b) => [b.textContent, b.getAttribute('aria-label')])).toEqual([['Add lap', null], ['Add pin', null]]);
  });

  it('one column: each row is the short button and its reason on one line, the row keeps its place when empty, full texts for assistive tech', async () => {
    vi.stubGlobal('matchMedia', () => ({ matches: false, addEventListener() {}, removeEventListener() {} }));
    s.settings = fresh({ ...ON, pins: [] });
    await start();
    const whys = [...document.querySelectorAll<HTMLElement>('.plus-row .edit-small')];
    expect(whys.map((n) => n.textContent)).toEqual([editTexts.selectFirst, '']); // one line for the box, in the first row
    expect(whys.map((n) => n.title)).toEqual([editTexts.selectFirst, '']);
    const add = document.querySelector<HTMLButtonElement>('.plus-item')!;
    expect(add.getAttribute('aria-describedby')).toBe(whys[0].id);
    expect(add.getAttribute('aria-label')).toBe('Add lap');
    await pick(2000);
    expect(whys.map((n) => n.textContent)).toEqual([expect.stringMatching(/^at [\d.]+ (mi|km)$/), expect.stringMatching(/^at /)]); // where each acts
    expect(whys.every((n) => n.isConnected)).toBe(true);
  });

  it('PN-12: "+" names where it acts; Add pin opens the dialog on Title, OK makes the pin, Cancel makes none, focus returns to "+"', async () => {
    s.settings = fresh({ ...ON, pins: [] });
    await start();
    await pick(2000);
    const sel = stubLog.chartStack[0].selection!;
    plusButton().click();
    expect(why()).toMatch(/^at [\d.]+ (mi|km)$/);
    sel.set(2000, 3000);
    s.frames.splice(0).forEach((f) => f(0));
    await settle();
    expect(why()).toMatch(/^[\d.]+–[\d.]+ (mi|km)$/);
    sel.clear();
    s.frames.splice(0).forEach((f) => f(0));
    await settle();
    button('Add pin').click();
    await dlgOpen();
    expect(dlg().querySelector('h2')!.textContent).toBe('Add pin');
    expect(dlg().querySelector('.edit-small')!.textContent).toMatch(/^at [\d.]+ (mi|km)$/);
    expect(document.activeElement).toBe(field('input'));
    expect(dlgButton('Delete pin').hidden).toBe(true);
    expect(field('input').placeholder).toBe('Point of interest');
    dlgButton('Cancel').click();
    expect(dlg().open).toBe(false);
    expect(rowTexts()).toHaveLength(0);
    expect(document.activeElement).toBe(plusButton());
    plusButton().click();
    button('Add pin').click();
    await dlgOpen();
    field('select').value = 'danger';
    field('select').dispatchEvent(new Event('change'));
    expect(field('input').placeholder).toBe('Danger');
    submit();
    expect(rowTexts()).toHaveLength(1);
    expect(document.activeElement).toBe(plusButton());
  });

  it('one column: the items are disabled, as "+" is, until a cursor or range is set', async () => {
    vi.stubGlobal('matchMedia', () => ({ matches: false, addEventListener() {}, removeEventListener() {} }));
    s.settings = fresh({ ...ON, pins: [] });
    await start();
    expect([...document.querySelectorAll<HTMLButtonElement>('.plus-item')].map((b) => b.disabled)).toEqual([true, true]);
    await pick(2000);
    expect([...document.querySelectorAll<HTMLButtonElement>('.plus-item')].map((b) => b.disabled)).toEqual([false, false]);
  });

  const why = (): string => (button('Add pin').nextElementSibling as HTMLElement).textContent!.trim();

  it('the button and its visible reason follow radius and trim changes; a twin is refused (r3-2 to r3-4)', async () => {
    s.settings = fresh({ ...ON, pins: [] });
    await start();
    expect([off('Add pin'), why()]).toEqual([true, '']);
    await pick(20); // inside the 300 m start radius
    expect([off('Add pin'), why()]).toEqual([true, 'Hidden or trimmed']);
    hideAt(0, 0); // radius off: the same cursor is public
    await settle();
    expect([off('Add pin'), why()]).toEqual([false, expect.stringMatching(/^at [\d.]+ (mi|km)$/)]); // where it will act
    hideAt(300, 300);
    await settle();
    expect(off('Add pin')).toBe(true);
    await pick(2000);
    type('trim_a', '3000'); // the cursor is now in the trimmed part
    await settle();
    expect([off('Add pin'), why()]).toEqual([true, 'Hidden or trimmed']);
    type('trim_a', '0');
    await settle();
    expect(off('Add pin')).toBe(false);
    button('Add pin').click();
    await dlgOpen();
    submit();
    expect(rowTexts()).toHaveLength(1);
    expect([off('Add pin'), why()]).toEqual([true, 'Already a pin here']);
    button('Add pin').click();
    expect(dlg().open).toBe(false);
    expect(rowTexts()).toHaveLength(1);
    await save();
    expect(patches().at(-1)).toEqual({ pins: [{ ...pin(2000, ''), type: 'point_of_interest' }] });
  });

  const row0 = (): HTMLButtonElement => document.querySelector<HTMLButtonElement>('.pin-row')!;
  const open0 = async (): Promise<void> => {
    row0().focus();
    row0().click();
    await dlgOpen();
  };

  it('edit via a row: filled in and selected while open; Cancel, Esc and the backdrop change nothing and deselect; OK applies; focus returns to the row (PIN-1, PIN-5)', async () => {
    s.settings = fresh({ ...ON, pins: [{ ...pin(1000, 'mid'), note: 'n1' }] });
    await start();
    expect(document.querySelector('.pin-item .pin-note')).toBeNull(); // the list is read-only
    await open0();
    expect(dlg().querySelector('h2')!.textContent).toBe('Edit pin');
    expect([field('select').value, field('input').value, dlg().querySelector('textarea')!.value]).toEqual(['danger', 'mid', 'n1']);
    expect(document.activeElement).toBe(field('select'));
    expect(document.querySelector('.pin-item.sel')).not.toBeNull();
    fill(field('input'), 'changed');
    dlgButton('Cancel').click();
    expect(document.querySelector('.pin-item.sel')).toBeNull();
    expect(rowTexts()[0]).toMatch(/^mid/);
    expect(document.activeElement).toBe(row0());
    await open0(); // the tap opens again: no toggle
    expect(field('input').value).toBe('mid'); // the draft of the last open is gone
    dlg().dispatchEvent(new Event('cancel')); // Esc and the back gesture end in close()
    dlg().close();
    expect(rowTexts()[0]).toMatch(/^mid/);
    await open0();
    dlg().dispatchEvent(new Event('pointerdown'));
    dlg().click(); // the backdrop
    expect(dlg().open).toBe(false);
    expect(rowTexts()[0]).toMatch(/^mid/);
    await open0();
    fill(field('input'), 'changed');
    submit();
    expect(rowTexts()[0]).toMatch(/^changed/);
    expect(document.activeElement).toBe(row0());
    await save();
    expect(patches()).toEqual([{ pins: [{ ...pin(1000, 'changed'), note: 'n1' }] }]);
  });

  it('a press that starts inside the dialog and ends on the backdrop does not close it', async () => {
    s.settings = fresh({ ...ON, pins: [pin(1000, 'mid')] });
    await start();
    await open0();
    field('input').dispatchEvent(new Event('pointerdown', { bubbles: true }));
    dlg().click(); // the drag ended on the backdrop
    expect(dlg().open).toBe(true);
    dlg().dispatchEvent(new Event('pointerdown', { bubbles: true }));
    dlg().click();
    expect(dlg().open).toBe(false);
  });

  it('edit via a map marker: focus returns to the marker; Esc in the dialog does not reach the fullscreen map', async () => {
    s.settings = fresh({ ...ON, pins: [pin(1000, 'mid')] });
    await start();
    const mk = document.querySelector<HTMLButtonElement>('.pin-mk')!;
    mk.focus();
    mk.click();
    await dlgOpen();
    const esc: string[] = [];
    const seen = (e: KeyboardEvent): number => esc.push(e.key);
    document.addEventListener('keydown', seen);
    dlg().dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    document.removeEventListener('keydown', seen);
    expect(esc).toEqual([]);
    submit();
    const now = document.querySelector('.pin-mk');
    expect(now).not.toBe(mk); // OK rebuilt the marker: focus is on its replacement
    expect(document.activeElement).toBe(now);
  });

  it('Delete in the dialog and the row ×: no confirmation; focus goes to the row now at the index, else the heading', async () => {
    s.settings = fresh({ ...ON, pins: [pin(1000, 'a'), pin(1500, 'b'), pin(2000, 'c')] });
    await start();
    const x = (): HTMLButtonElement[] => [...document.querySelectorAll<HTMLButtonElement>('.pin-item .added-x-btn')];
    expect(x().map((b) => b.title)).toEqual(['Delete pin: a', 'Delete pin: b', 'Delete pin: c']);
    x()[1].click();
    expect(rowTexts().map((r) => r[0])).toEqual(['a', 'c']);
    expect(document.activeElement).toBe(document.querySelectorAll('.pin-row')[1]);
    await open0();
    dlgButton('Delete pin').click();
    expect(rowTexts().map((r) => r[0])).toEqual(['c']);
    expect(document.activeElement).toBe(row0());
    x()[0].click();
    expect(rowTexts()).toHaveLength(0);
    expect(document.activeElement).toBe(document.querySelector('.pins-head'));
    await save();
    expect(patches()).toEqual([{ pins: [] }]);
  });
});

