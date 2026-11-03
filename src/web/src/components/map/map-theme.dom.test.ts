// @vitest-environment jsdom
// Map colours from the theme's custom properties and the scheme's style URL (F-14 design D-10, D-12).
import { describe, expect, it } from 'vitest';
import {
  CLASS_COLOURS,
  MAP_COLOURS,
  OPENFREEMAP_DARK_STYLE_URL,
  OPENFREEMAP_STYLE_URL,
  TILE_HOST,
  baseStyle,
  classColourExpression,
  classColours,
  mapColours,
  rangeColourExpression,
  recolour,
  styleUrlFor,
} from './map-style';

describe('mapColours', () => {
  it('falls back to the light values where no theme is present', () => {
    expect(mapColours(document.createElement('div'))).toEqual(MAP_COLOURS);
  });
  it('reads the theme from the element', () => {
    const el = document.createElement('div');
    el.style.setProperty('--route', '#ff5cae');
    el.style.setProperty('--map-bg', '#1b1c1f');
    const c = mapColours(el);
    expect(c.route).toBe('#ff5cae');
    expect(c.background).toBe('#1b1c1f');
    expect(c.casing).toBe(MAP_COLOURS.casing);
  });
  it('baseStyle uses the given colours', () => {
    const style = baseStyle({ recorded: [[[8, 47], [8.1, 47.1]]], bridges: [] }, [], { ...MAP_COLOURS, route: '#123456' });
    const line = style.layers.find((l) => l.id === 'wx-route-line');
    expect((line?.paint as Record<string, unknown>)['line-color']).toBe('#123456');
  });
});

describe('recolour', () => {
  it('repaints the layers that exist, once per property', () => {
    const calls: string[] = [];
    const have = new Set(['background', 'wx-route-line', 'wx-start']);
    recolour(
      { getLayer: (id) => (have.has(id) ? {} : undefined), setPaintProperty: (id, name, value) => calls.push(`${id}.${name}=${value}`) },
      { ...MAP_COLOURS, route: '#aa0000' },
    );
    expect(calls).toContain('wx-route-line.line-color=#aa0000');
    expect(calls).toContain('background.background-color=' + MAP_COLOURS.background);
    expect(calls.some((c) => c.startsWith('wx-finish'))).toBe(false);
  });
  it('F-25 C-5: rebuilds the class colour expressions from the theme: route colour fallback; the range expression, outline colour', () => {
    const calls: [string, string, unknown][] = [];
    const classes = CLASS_COLOURS.map((_, k) => '#0000' + (10 + k));
    recolour({ getLayer: () => ({}), setPaintProperty: (id, name, value) => calls.push([id, name, value]) }, { ...MAP_COLOURS, route: '#aa0000' }, classes);
    expect(calls).toContainEqual(['wx-colour-line', 'line-color', classColourExpression(classes, '#aa0000')]);
    expect(calls).toContainEqual(['wx-range-line', 'line-color', rangeColourExpression(classes, { ...MAP_COLOURS, route: '#aa0000' })]);
    expect(calls).toContainEqual(['wx-range-casing', 'line-color', MAP_COLOURS.rangeOutline]);
    expect(calls.filter(([id]) => id === 'wx-range-line')).toHaveLength(1);
  });
});

describe('classColours (F-25 C-5)', () => {
  it('falls back to the light values; reads --seq-k and --div-k from the element', () => {
    expect(classColours(document.createElement('div'))).toEqual(CLASS_COLOURS);
    const el = document.createElement('div');
    el.style.setProperty('--seq-6', '#010203');
    el.style.setProperty('--div-0', '#040506');
    const c = classColours(el);
    expect([c[6], c[7], c[8]]).toEqual(['#010203', '#040506', CLASS_COLOURS[8]]);
  });
});

describe('style URLs', () => {
  it('liberty for light, dark for dark; both on the one tile host', () => {
    expect(styleUrlFor('light')).toBe(OPENFREEMAP_STYLE_URL);
    expect(styleUrlFor('dark')).toBe(OPENFREEMAP_DARK_STYLE_URL);
    expect(OPENFREEMAP_DARK_STYLE_URL.startsWith(TILE_HOST + '/')).toBe(true);
  });
});
