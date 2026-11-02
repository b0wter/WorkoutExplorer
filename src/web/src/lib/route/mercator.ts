// Web Mercator on the unit square (F-06 design, "Route and markers"). MapLibre's world is
// 512 · 2^zoom CSS px wide in these units (pitch 0, Mercator), which the hit test uses
// to turn pixel radii into Mercator distances (ADR-015 decision 5).

/** Latitude limit of Web Mercator in degrees; beyond it y is clamped. */
export const MAX_MERCATOR_LAT = 85.051129;

/** x = (lon + 180) / 360: 0 at −180°, 1 at 180°. */
export function mercatorX(lon: number): number {
  return (lon + 180) / 360;
}

/** y = (1 − ln(tan(π/4 + φ/2)) / π) / 2, φ clamped to ±MAX_MERCATOR_LAT: about 0 at the north limit, 1 at the south. */
export function mercatorY(lat: number): number {
  const clamped = lat > MAX_MERCATOR_LAT ? MAX_MERCATOR_LAT : lat < -MAX_MERCATOR_LAT ? -MAX_MERCATOR_LAT : lat;
  const phi = (clamped * Math.PI) / 180;
  return (1 - Math.log(Math.tan(Math.PI / 4 + phi / 2)) / Math.PI) / 2;
}
