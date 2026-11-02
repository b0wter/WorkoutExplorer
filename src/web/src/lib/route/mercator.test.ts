// Web Mercator (F-06 design, Tests).

import { describe, expect, it } from 'vitest';
import { MAX_MERCATOR_LAT, mercatorX, mercatorY } from './index';

describe('mercator', () => {
  it('(0, 0) → (0.5, 0.5)', () => {
    expect(mercatorX(0)).toBe(0.5);
    expect(mercatorY(0)).toBeCloseTo(0.5, 15);
  });

  it('longitude −180 / 180 → 0 / 1', () => {
    expect(mercatorX(-180)).toBe(0);
    expect(mercatorX(180)).toBe(1);
  });

  it('latitude ±85.051129 → about 0 / 1, beyond it clamped', () => {
    expect(mercatorY(MAX_MERCATOR_LAT)).toBeCloseTo(0, 6);
    expect(mercatorY(-MAX_MERCATOR_LAT)).toBeCloseTo(1, 6);
    expect(mercatorY(89.9)).toBe(mercatorY(MAX_MERCATOR_LAT));
    expect(mercatorY(-90)).toBe(mercatorY(-MAX_MERCATOR_LAT));
  });

  it('strictly increasing in longitude, strictly decreasing in latitude', () => {
    let prevX = -Infinity;
    let prevY = Infinity;
    for (let d = -85; d <= 85; d += 0.25) {
      const x = mercatorX(d * 2);
      const y = mercatorY(d);
      expect(x).toBeGreaterThan(prevX);
      expect(y).toBeLessThan(prevY);
      prevX = x;
      prevY = y;
    }
    // Neighbouring 10⁻⁶° steps stay distinct (float64).
    expect(mercatorY(47.000001)).toBeLessThan(mercatorY(47));
    expect(mercatorX(8.000001)).toBeGreaterThan(mercatorX(8));
  });
});
