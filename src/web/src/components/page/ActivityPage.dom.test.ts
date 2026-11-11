// @vitest-environment jsdom
// The page modules as Svelte components (F-13 design D-9), rendered from the committed samples.
// Covers AC 1 (static content), 3, 4, 5, 8 and 10 for the edit page's rendering of the markup.
import { afterEach, describe, expect, it } from 'vitest';
import { renderSample, type Fixture } from '../../page/testing/fixture';

const NOTICE = 'Parts of the route, such as the start and end, may be hidden for privacy. Totals include them.';
let fx: Fixture | null = null;
afterEach(() => {
  fx?.remove();
  fx = null;
  document.head.innerHTML = '';
});
const text = (sel: string): string => fx?.target.querySelector(sel)?.textContent ?? '';

describe('ActivityPage', () => {
  it('AC 1: title, sport, local start, description, expiry estimate, summary and notice are in the markup', () => {
    fx = renderSample('ride-30min');
    expect(text('h1')).toBe('Morning loop');
    expect(text('.when')).toBe('Ride · 23 Jun 2026, 07:00');
    expect(text('.desc')).toBe('Easy spin.\nSecond line.');
    expect(text('time')).toBe('Expires around 30 Oct 2026');
    expect(fx.target.querySelector('time')?.getAttribute('datetime')).toBe('2026-10-30T09:15:00Z');
    expect(fx.target.querySelectorAll('.metric').length).toBe(fx.model.Summary.length);
    expect(text('.notice')).toBe(NOTICE);
  });

  it('AC 3: a run shows no power cell; a file without heart rate shows no heart rate and no "0 bpm"', () => {
    fx = renderSample('run-30min');
    expect(fx.target.textContent).not.toMatch(/power|\bW\b/i);
    fx.remove();
    fx = renderSample('ride-no-hr');
    expect(fx.target.textContent).not.toMatch(/bpm|heart|\bHR\b/i);
  });

  it('AC 4: the privacy notice is identical for every sample, whatever is hidden', () => {
    for (const name of ['ride-30min', 'indoor-ride', 'hostile-text', 'triathlon']) {
      fx = renderSample(name);
      expect(text('.notice'), name).toBe(NOTICE);
      fx.remove();
    }
    fx = null;
  });

  it('F-43: the device line sits between the summary and the notice with the model text; none without one', () => {
    fx = renderSample('triathlon');
    const line = fx.target.querySelector('.summary + p.device + p.notice')?.previousElementSibling;
    expect(line?.textContent).toBe(fx.model.Device);
    expect(line?.children.length).toBe(0);
    fx.remove();
    fx = renderSample('ride-no-laps');
    expect(fx.target.querySelector('.device')).toBeNull();
  });

  it('AC 5: HTML and script in the title and description appear as literal text', () => {
    fx = renderSample('hostile-text');
    expect(fx.target.querySelector('script, b, i, img')).toBeNull();
    expect(text('h1')).toBe('<b>Bold</b> & "quoted" <script>alert(1)</script>');
    expect(text('.desc')).toBe('<script>alert(2)</script>\n<img src=x onerror=alert(3)> &amp; done');
    expect(text('.device')).toBe('Recorded with <i>Dev</i> &amp; "x"');
  });

  it('AC 8: an indoor workout shows the hint and no map slot; the charts slot is reserved', () => {
    fx = renderSample('indoor-ride');
    expect(text('.hint')).toBe('No location data in this workout.');
    expect(fx.target.querySelector('#map-slot')).toBeNull();
    expect(fx.target.querySelector('#charts-slot')?.className).toBe('charts ' + fx.model.ChartsClass);
    expect(fx.target.querySelectorAll('.metric').length).toBeGreaterThan(0);
  });

  it('a route reserves the map slot and shows no hint; slots are empty before the bundle runs', () => {
    fx = renderSample('ride-12h');
    expect(fx.target.querySelector('#map-slot')?.childNodes.length).toBe(0);
    expect(fx.target.querySelector('#charts-slot')?.childNodes.length).toBe(0);
    expect(fx.target.querySelector('.hint')).toBeNull();
  });

  it('laps: one row per lap; no module without laps', () => {
    fx = renderSample('ride-12h');
    expect(fx.target.querySelectorAll('tbody tr')).toHaveLength(24);
    expect(fx.target.querySelectorAll('thead th')).toHaveLength(fx.model.Laps!.Columns.length);
    expect(fx.target.querySelector('tbody tr td')?.textContent).toBe('1');
  });

  it('F-18 L-8: distance without laps shows "No laps recorded." and no table', () => {
    fx = renderSample('ride-no-laps');
    expect(fx.target.querySelector('.laps .laps-none')?.textContent).toBe('No laps recorded.');
    expect(fx.target.querySelector('.laps table')).toBeNull();
  });

  it('AC 10: the footer has About; Report (a hidden button, F-28 RF-1) only where asked (the edit page has none)', () => {
    fx = renderSample('ride-30min', { report: true });
    expect(fx.target.querySelector('footer a[href="/about"]')?.textContent).toBe('About');
    expect(fx.target.querySelector('footer button[data-report]')?.textContent).toBe('Report');
    fx.remove();
    document.head.innerHTML = '';
    // ActivityPage defaults to no Report link (the edit page, question 21).
    fx = renderSample('ride-30min', { report: false });
    expect(fx.target.querySelector('footer button[data-report]')).toBeNull();
    expect(fx.target.querySelector('footer a[href="/about"]')).not.toBeNull();
  });

  it('the copy link is a real link to the view path (works without JavaScript)', () => {
    fx = renderSample('ride-30min');
    expect(fx.target.querySelector('a[data-copy-link]')?.getAttribute('href')).toBe('/w/ride-30min');
  });

  it('layout (D-17): the wrapper holds header to footer (the footer ends the right column); has-map only with a map', () => {
    fx = renderSample('ride-30min');
    const page = fx.target.querySelector('.page');
    expect(page?.classList.contains('has-map')).toBe(true);
    expect(fx.target.querySelector('.layout #map-slot')).not.toBeNull();
    expect(fx.target.querySelector('.layout .laps')).not.toBeNull();
    expect(page?.querySelector(':scope > .layout > .foot:last-child')).not.toBeNull();
    expect(fx.target.querySelector('.map-pin')).toBeNull();
    fx.remove();
    document.head.innerHTML = '';
    fx = renderSample('indoor-ride');
    expect(fx.target.querySelector('.page')?.classList.contains('has-map')).toBe(false);
  });

  it('a fully hidden route (AP-3): one column, no map slot, no hint, nothing to pin', () => {
    fx = renderSample('ride-30min', { route: { Map: '', NoLocation: '', Hidden: '' } });
    expect(fx.target.querySelector('.page')?.classList.contains('has-map')).toBe(false);
    expect(fx.target.querySelector('#map-slot')).toBeNull();
    expect(fx.target.querySelector('.hint')).toBeNull();
    expect(fx.target.querySelector('.map-pin')).toBeNull();
  });
});
