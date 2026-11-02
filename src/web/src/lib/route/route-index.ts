// Nearest positioned sample to a point on the map (F-06 design, "Route interaction";
// ADR-015 decision 5; review decision 5: nearest sample, earliest on a tie).
//
// A uniform grid over the route's Web Mercator bounding box, about 8 samples per cell,
// square cells, stored as two Uint32Arrays (cell start offsets, items). A counting sort
// fills it in route order, so every cell lists its samples in ascending sample index.
// Queries allocate nothing.

import type { Route } from './route';

export interface RouteIndex {
  /** Sample index of the nearest positioned sample to Mercator point (x, y) within radius;
   *  samples within tie of the nearest distance count as tied and the lowest sample index
   *  wins; −1 if none within radius. radius, tie in Mercator units (> 0). No allocation. */
  nearest(x: number, y: number, radius: number, tie: number): number;
}

/** Target number of samples per grid cell. */
const PER_CELL = 8;

export function createRouteIndex(route: Route): RouteIndex {
  const { indices, mx, my } = route;
  const m = indices.length;

  let minX = Infinity;
  let minY = Infinity;
  let maxX = -Infinity;
  let maxY = -Infinity;
  for (let k = 0; k < m; k++) {
    if (mx[k] < minX) minX = mx[k];
    if (mx[k] > maxX) maxX = mx[k];
    if (my[k] < minY) minY = my[k];
    if (my[k] > maxY) maxY = my[k];
  }
  const w = maxX - minX;
  const h = maxY - minY;
  const target = Math.max(1, Math.ceil(m / PER_CELL));
  // Square cells. The second term keeps a very thin box (a straight north–south line) from
  // getting millions of cells along its long side: no side gets more than `target` cells.
  let size = Math.max(Math.sqrt((w * h) / target), Math.max(w, h) / target);
  if (!(size > 0)) size = 1; // all samples at one point
  const cols = Math.max(1, Math.min(target, Math.ceil(w / size)));
  const rows = Math.max(1, Math.min(target, Math.ceil(h / size)));

  const cellX = (x: number): number => {
    const c = Math.floor((x - minX) / size);
    return c < 0 ? 0 : c >= cols ? cols - 1 : c;
  };
  const cellY = (y: number): number => {
    const r = Math.floor((y - minY) / size);
    return r < 0 ? 0 : r >= rows ? rows - 1 : r;
  };

  // Counting sort: start[c] .. start[c + 1] are the items of cell c.
  const start = new Uint32Array(cols * rows + 1);
  const cellOf = new Uint32Array(m);
  for (let k = 0; k < m; k++) {
    const c = cellY(my[k]) * cols + cellX(mx[k]);
    cellOf[k] = c;
    start[c + 1]++;
  }
  for (let c = 0; c < cols * rows; c++) start[c + 1] += start[c];
  const items = new Uint32Array(m);
  const fill = start.slice(0, cols * rows);
  for (let k = 0; k < m; k++) items[fill[cellOf[k]]++] = k;

  return {
    nearest(x: number, y: number, radius: number, tie: number): number {
      if (!(radius > 0) || x + radius < minX || x - radius > maxX || y + radius < minY || y - radius > maxY) return -1;
      const c0 = cellX(x - radius);
      const c1 = cellX(x + radius);
      const r0 = cellY(y - radius);
      const r1 = cellY(y + radius);
      const r2 = radius * radius;

      // Pass 1: the smallest squared distance within the radius.
      let best = Infinity;
      for (let r = r0; r <= r1; r++) {
        for (let c = c0; c <= c1; c++) {
          const cell = r * cols + c;
          for (let p = start[cell]; p < start[cell + 1]; p++) {
            const k = items[p];
            const dx = mx[k] - x;
            const dy = my[k] - y;
            const d2 = dx * dx + dy * dy;
            if (d2 <= r2 && d2 < best) best = d2;
          }
        }
      }
      if (best === Infinity) return -1;

      // Pass 2: the lowest route position (= lowest sample index, indices ascend) within
      // tie of the best distance and within the radius. max(best, …) keeps the best sample
      // itself eligible whatever the rounding of (√best + tie)².
      const limit = Math.sqrt(best) + tie;
      const threshold = Math.max(best, Math.min(limit * limit, r2));
      let lowest = m;
      for (let r = r0; r <= r1; r++) {
        for (let c = c0; c <= c1; c++) {
          const cell = r * cols + c;
          for (let p = start[cell]; p < start[cell + 1]; p++) {
            const k = items[p];
            if (k >= lowest) break; // ascending within a cell
            const dx = mx[k] - x;
            const dy = my[k] - y;
            if (dx * dx + dy * dy <= threshold) lowest = k;
          }
        }
      }
      return indices[lowest];
    },
  };
}
