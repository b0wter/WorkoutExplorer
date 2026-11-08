// The hard rules of the activity page as checks (F-13 design D-11; ADR-026 item 5; CSP, ADR-014):
// no inline script or style in the template, no style attributes, no {@html}.
import { describe, expect, it } from 'vitest';
import template from '../../index.html?raw';
import { readWebFile } from '../../test-support/source.mjs';
import { DEFAULT_CHART_HEIGHT, chartAreaHeight } from '../components/charts/uplot-options';

const sources = import.meta.glob(['./*.ts', '../components/page/*.svelte', '../components/edit/*', '../edit/*', '../lib/page/*.ts', '../lib/format/*.ts', '!**/*.test.ts', '!**/samples.ts'], {
  query: '?raw',
  import: 'default',
  eager: true,
}) as Record<string, string>;

describe('template', () => {
  it('separates value and unit like SummaryStrip.svelte does (ADR-026 drift)', () => {
    const svelte = sources['../components/page/SummaryStrip.svelte'];
    expect(template).toContain('<span class="n">{{.Value}}</span>{{if .Unit}}&nbsp;<span class="u">');
    expect(svelte).toContain('<span class="n">{item.Value}</span>{#if item.Unit}&nbsp;<span class="u">');
  });

  it('has no inline script but the theme script, no inline style, no style attribute, no on* handler', () => {
    expect(template).not.toMatch(/<style[\s>]/i);
    // DS-4: the one allowed by its CSP hash (ADR-032 item 3), checked in theme-script.test.ts.
    expect(template.match(/<script(?![^>]*\ssrc=)[^>]*>/gi)).toEqual(['<script>']);
    expect(template).not.toMatch(/\sstyle\s*=/i);
    expect(template).not.toMatch(/\son[a-z]+\s*=/i);
  });

  it('prints only fields: no template functions or pipelines (ADR-026 item 1)', () => {
    const actions = [...template.matchAll(/\{\{(.*?)\}\}/gs)].map((m) => m[1].trim());
    for (const a of actions) {
      expect(a, a).toMatch(/^(\.[A-Za-z.]*|if \.[A-Za-z.]+|else|end|range \.[A-Za-z.]*|with \.[A-Za-z.]+)$/);
    }
  });

  it('marks the map case from Route.Map only and has no pin toggle (D-17, D-19)', () => {
    expect(template).toContain('<div class="page{{if .Route.Map}} has-map{{end}}">');
    expect(template).toMatch(/<div class="layout">[\s\S]*<footer class="foot">[\s\S]*<\/footer>\s*<\/div>/);
    expect(template).not.toMatch(/map-pin|aria-pressed|pinned/);
    // The one button: Report, hidden without JavaScript (F-28 RF-1, RF-9).
    expect(template.match(/<button[^>]*>/g)).toEqual(['<button type="button" data-report hidden>']);
  });

  it('has an empty laps heading slot and no switch, like LapsTable.svelte (F-18 L-6, AC 6)', () => {
    const slot = '<div class="laps-head"><h2>Laps</h2><span class="laps-slot" id="laps-slot"></span></div>';
    expect(template).toContain(slot);
    expect(sources['../components/page/LapsTable.svelte']).toContain(slot);
    expect(template).not.toMatch(/laps-switch|Splits/);
  });

  it('carries the head of D-11', () => {
    expect(template).toContain('<html lang="en">');
    expect(template).toContain('<meta name="referrer" content="no-referrer"');
    expect(template).toContain('<meta name="robots" content="noindex, nofollow, noarchive"');
    expect(template).toContain('<link rel="preload" id="wx-data" href="{{.Page.DataURL}}" as="fetch" crossorigin');
  });
});

describe('sources', () => {
  it('never use {@html}, style attributes or directives, or inline handlers', () => {
    expect(Object.keys(sources).length).toBeGreaterThan(5);
    for (const [file, code] of Object.entries(sources)) {
      if (file.endsWith('.test.ts')) continue;
      expect(code, file).not.toMatch(/\{@html/);
      expect(code, file).not.toMatch(/\sstyle[:=]/);
      expect(code, file).not.toMatch(/\son[a-z]+=\{/);
      expect(code, file).not.toMatch(/Intl\.[A-Z]|\.toLocale/);
    }
  });
});

describe('reserved sizes (D-10, AC 7: no layout shift)', () => {
  const css = readWebFile('src/page/page.css');
  const rule = (selector: string): string => {
    const m = css.match(new RegExp('^' + selector.replace(/[.]/g, '\\.') + ' \\{([^}]*)\\}', 'm'));
    if (!m) throw new Error('no rule ' + selector);
    return m[1];
  };

  it.each([1, 2, 3, 4, 5, 6])('.charts-%s reserves exactly what ChartStack sets for its chart area', (n) => {
    expect(rule('.charts-' + n)).toContain(`--area: ${chartAreaHeight(n, DEFAULT_CHART_HEIGHT)}px`);
  });

  it.each([1, 2, 3, 4, 5, 6, 7, 8])('.cells-%s reserves the readout rows of a 4-column grid (8 columns: one row)', (k) => {
    expect(rule('.cells-' + k)).toBe(` --r4: ${Math.ceil(k / 4)}; `);
  });

  it('the readout shares the range panel card; 29 px rows and its 8 px of padding are in the reserved height', () => {
    const shared = css.match(/^#charts-slot \.readout,\n#map-slot \.readout,\n\.range-panel \{([^}]*)\}/m)?.[1] ?? '';
    for (const d of ['display: grid', 'padding: 4px 10px', 'background: var(--card)', 'border-radius: 6px']) expect(shared, d).toContain(d);
    expect(shared).not.toMatch(/row-gap|(?<!column-)gap:/);
    expect(css).toMatch(/^\.readout \.label,\n\.range-panel \.label \{[^}]*line-height: 13px;/m);
    expect(css).toMatch(/^\.readout \.val,\n\.range-panel \.val \{[^}]*line-height: 16px;/m);
    expect(css).toMatch(/^\.readout \.unit,\n\.range-panel \.unit \{[^}]*line-height: 13px;/m);
    expect(css).toMatch(/^#charts-slot \.readout,\n#map-slot \.readout \{\s*grid-template-columns: repeat\(4, minmax\(0, 1fr\)\);/m); // F-33: the strip shares the tracks
    expect(css).toMatch(/@media \(min-width: 640px\) \{\s*\.charts \{\s*--r: 1;\s*\}\s*#charts-slot \.readout,\s*#map-slot \.readout \{\s*grid-template-columns: repeat\(8, minmax\(0, 1fr\)\);/);
    expect(rule('.charts')).toContain('calc(var(--bar) + var(--r) * 29px + 8px + var(--area) + var(--hint))');
  });

  it('F-17: the hint row is always reserved with charts, the axis toolbar row only with has-distance', () => {
    expect(rule('.charts')).toMatch(/--bar: 0px;[^}]*--hint: 24px;/);
    expect(rule('.charts.has-distance')).toContain('--bar: 32px');
    const stack = readWebFile('src/components/charts/ChartStack.svelte');
    expect(stack).toMatch(/\.toolbar \{\s*height: 32px;/);
    expect(stack).toMatch(/\.select-hint \{\s*height: 20px;\s*margin: 4px 0 0;/);
  });

  it('F-17 R-9: the range slot is empty and zero height until filled, sticky at the bottom in one column', () => {
    expect(rule('.range-slot')).toMatch(/position: sticky;\s*bottom: 0;/);
    expect(rule('.range-slot:empty')).toContain('padding: 0');
  });

  it('summary cells size to their content: no fixed height or overflow (max line is never clipped)', () => {
    for (const sel of ['.summary', '.metric', '.k', '.v', '.max']) expect(rule(sel), sel).not.toMatch(/height|overflow/);
  });

  it('the copy-link fallback field takes its own header line, not the title row', () => {
    expect(rule('.head')).toContain('flex-wrap: wrap');
    expect(rule('.copy-url')).toContain('flex: 1 0 100%');
  });

  it('the header controls wrap below the title at fixed widths, so the title never gets squeezed', () => {
    expect(rule('.head-text')).toContain('min-width: min(160px, 100% - 44px)');
    expect(css).toMatch(/@media \(min-width: 560px\) \{\s*\.head-text \{\s*min-width: min\(240px, 100% - 44px\);/);
    expect(rule('.units-slot,\n.theme-slot')).toMatch(/width: 104px;\s*height: 32px;/);
    expect(rule('.copy')).toMatch(/min-width: 84px;\s*height: 32px;/);
  });

  it('viewport units only in the desktop and landscape maps and the pinned phone map, never dvh/svh', () => {
    const rest = css
      .replace(/(\.has-map \.map-slot \{[^}]*\})/g, (m) => m.replace(/height: 100l?vh;/g, ''))
      .replace(/--pin-h: 35l?vh;/g, '')
      .replace('@supports (height: 1lvh)', '');
    expect(rest).not.toMatch(/\d(vh|dvh|svh|lvh)\b/);
    expect(css).toMatch(/\.has-map \.map-slot \{[^}]*height: 100vh;\s*height: 100lvh;/);
    expect(css).not.toMatch(/dvh|svh/);
  });
});

describe('page.css layout structure (D-17 to D-19)', () => {
  const css = readWebFile('src/page/page.css');
  const block = (query: string): string => {
    const i = css.indexOf(query);
    expect(i, query).toBeGreaterThan(-1);
    return css.slice(i, css.indexOf('\n}\n', i));
  };

  it('two columns only from 1024 px with a map; the right column is at least 608 px', () => {
    const two = block('@media (min-width: 1024px) {');
    expect(two).toMatch(/\.has-map \.layout \{\s*display: grid;/);
    expect(two).toContain('minmax(608px, 1fr)');
    expect(two).toMatch(/\.has-map \.map-slot \{[^}]*position: sticky;/);
    expect(two).toMatch(/\.map-pin-group \{\s*display: none;/);
    expect(css).not.toMatch(/^\.layout \{[^}]*display: grid/m);
  });

  const PORTRAIT = '@media (max-width: 1023.98px) and (orientation: portrait), (max-width: 1023.98px) and (min-height: 500.02px) {';
  const LANDSCAPE = '@media (max-width: 1023.98px) and (orientation: landscape) and (max-height: 500px) {';

  it('the pinned map is a portrait-phone rule (never a short landscape screen) and the readout sticks under it', () => {
    const pin = block(PORTRAIT);
    expect(pin).toMatch(/\.layout\.pinned \.map-slot \{[^}]*position: sticky;[^}]*top: 0;/);
    expect(pin).toMatch(/\.layout\.pinned #charts-slot \.readout \{\s*top: var\(--pin-h\);/);
    expect(css.indexOf('.pinned')).toBeGreaterThan(css.indexOf(PORTRAIT));
    expect(block(LANDSCAPE)).not.toContain('.pinned');
  });

  it('pinned height: 35lvh with a 35vh fallback (at most 40 %), no fixed short-landscape height, no dvh/svh', () => {
    const phone = css.slice(css.indexOf(PORTRAIT), css.indexOf('@media (min-width: 1024px) {'));
    expect(phone).not.toMatch(/dvh|svh/);
    // 35vh is the base value; 35lvh applies only where lvh is supported (a real fallback).
    expect(phone).toMatch(/\.layout \{\s*--pin-h: 35vh;\s*\}\s*@supports \(height: 1lvh\) \{\s*\.layout \{\s*--pin-h: 35lvh;/);
    const pct = [...phone.matchAll(/--pin-h: (\d+)l?vh;/g)].map((m) => Number(m[1]));
    expect(Math.max(...pct)).toBeLessThanOrEqual(40);
    expect(css).not.toMatch(/--pin-h: \d+px/);
  });

  it('F-17: short landscape with a map is two columns, 40 % map fixed full height, no pin toggle, 4-column readout', () => {
    const land = block(LANDSCAPE);
    expect(land).toMatch(/\.has-map \.layout \{\s*padding-left: var\(--map-w, 40%\);/);
    expect(land).toMatch(/\.has-map \.map-slot \{[^}]*position: fixed;[^}]*width: var\(--map-w, 40%\);[^}]*height: 100lvh;/);
    expect(land).toMatch(/\.map-pin-group \{\s*display: none;/);
    expect(land).toMatch(/\.has-map #charts-slot \.readout \{\s*grid-template-columns: repeat\(4, minmax\(0, 1fr\)\);/);
    expect(land).toMatch(/\.has-map \.charts \{\s*--r: var\(--r4\);/);
    expect(land).not.toContain('display: grid'); // one block flow: the range slot can stick
  });

  it('F-17: from 1024 px with a map the range panel is in place, not sticky', () => {
    expect(block('@media (min-width: 1024px) {')).toMatch(/\.has-map \.layout > \.range-slot \{\s*position: static;/);
  });
});

describe('theme (F-14 D-9, AC 7, AC 8)', () => {
  const css = readWebFile('src/page/page.css');
  const block = (re: RegExp): string => css.match(re)?.[1] ?? '';
  const light = block(/^:root \{([^}]*)\}/m);
  const dark = block(/@media \(prefers-color-scheme: dark\) \{\s*:root:not\(\[data-theme='light'\]\) \{([^}]*)\}/);
  const forced = block(/^:root\[data-theme='dark'\] \{\s*color-scheme: dark;([^}]*)\}/m);
  const values = (b: string): string => b.replace(/\s+/g, ' ').trim();
  const names = (b: string): string[] => [...b.matchAll(/(--[a-z_-]+):/g)].map((m) => m[1]);

  it('every colour is a custom property; the dark block overrides only existing ones', () => {
    expect(names(light).length).toBeGreaterThan(20);
    expect(names(dark).length).toBeGreaterThan(20);
    for (const n of names(dark)) expect(names(light), n).toContain(n);
    expect(light).toContain('color-scheme: light dark');
  });

  it('a forced theme (DS-4) wins both ways; the forced dark block repeats the system dark block exactly', () => {
    expect(values(forced)).toBe(values(dark));
    expect(css).toMatch(/^:root\[data-theme='light'\] \{\s*color-scheme: light;\s*\}/m);
    const edit = readWebFile('src/edit/edit.css');
    const editDark = edit.match(/:root:not\(\[data-theme='light'\]\) \{([^}]*)\}/)?.[1] ?? '';
    expect(values(editDark)).toContain('--hidden');
    expect(values(edit.match(/^:root\[data-theme='dark'\] \{([^}]*)\}/m)?.[1] ?? '')).toBe(values(editDark));
    expect(css + edit).not.toMatch(/prefers-color-scheme: dark\) \{\s*:root \{/);
  });

  it('the rest of the stylesheet has no colour literal', () => {
    const rest = css
      .replace(/^:root \{[^}]*\}/m, '')
      .replace(/@media \(prefers-color-scheme: dark\) \{\s*:root:not\(\[data-theme='light'\]\) \{[^}]*\}\s*\}/, '')
      .replace(/^:root\[data-theme='dark'\] \{[^}]*\}/m, '');
    expect(rest).not.toMatch(/#[0-9a-fA-F]{3,8}\b|rgba?\(/);
  });

  it('the lazy map CSS has no colour literal either', () => {
    expect(readWebFile('src/components/map/map-view.css')).not.toMatch(/#[0-9a-fA-F]{3,8}\b|rgba?\(/);
  });

  it('both pages declare the colour scheme in the head, so the first paint follows the system', () => {
    expect(template).toContain('<meta name="color-scheme" content="light dark"');
    expect(readWebFile('upload.html')).toContain('<meta name="color-scheme" content="light dark"');
  });

  it('the upload page has its own light and dark theme, CSS only (shared with the generic page, F-15 P-12)', () => {
    expect(readWebFile('src/upload/upload.css')).toContain("@import '../theme/plain.css';");
    const theme = readWebFile('src/theme/plain.css');
    expect(theme).toContain('@media (prefers-color-scheme: dark)');
    expect(theme).toContain('color-scheme: light dark');
  });

  it('browser storage only through lib/prefs, localStorage only; no cookie, URL or history state (DS-1, AP-5, AP-17)', () => {
    const all = import.meta.glob(['../**/*.ts', '../**/*.svelte', '!../**/*.test.ts', '!../**/testing/**', '!../**/*.test-helper.*'], {
      query: '?raw',
      import: 'default',
      eager: true,
    }) as Record<string, string>;
    expect(Object.keys(all).length).toBeGreaterThan(40);
    for (const [file, text] of Object.entries(all)) {
      // F-26 (ADR-038 item 3): the fullscreen map's one entry holds no state and no URL.
      const code = file === '../components/map/map-view.ts' ? text.replace("history.pushState(null, '')", '') : text;
      expect(code, file).not.toMatch(/sessionStorage|document\.cookie|cookieStore|indexedDB|history\.(push|replace)State/);
      if (file === '../lib/prefs/index.ts') continue;
      // The page glue hands lib/prefs its storage getter, nothing else.
      const rest = file === './start.ts' ? code.replace('(): KeyValueStore => win.localStorage', '') : code;
      expect(rest, file).not.toMatch(/localStorage|\.(getItem|setItem|removeItem)\(/);
    }
    expect(all['./start.ts']).toContain('createViewerPrefs((): KeyValueStore => win.localStorage)');
  });
});
