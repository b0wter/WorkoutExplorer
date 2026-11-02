// Min/max per pixel column with breaks (F-05 design, "Breaks and decimation";
// TD Workout data file, Charts; ADR-009 item 2). Pure: reads WorkoutData
// arrays, never changes them, allocates only the result.
//
// A segment is a maximal run of consecutive samples that are not missing and
// do not cross a gap. A break (y = null) separates two segments. Nothing is
// ever drawn as zero, filled or interpolated.

/** uPlot AlignedData parts for one series. */
export interface Decimated {
  readonly x: number[];
  readonly y: (number | null)[];
}

/** x non-decreasing where present, NaN = no x (a break: the distance axis, F-17 R-7); y with
 *  NaN = missing; gaps = WorkoutData.gaps; [xMin, xMax] = the axis range; buckets ≥ 1. Output x
 *  is non-decreasing (equal values allowed: a stretch without progress folds into one column).
 *  Pure; allocates only the result. */
export function decimate(
  x: Float64Array,
  y: Float64Array,
  gaps: Uint32Array,
  xMin: number,
  xMax: number,
  buckets: number,
): Decimated {
  const outX: number[] = [];
  const outY: (number | null)[] = [];
  const n = Math.min(x.length, y.length);
  const span = xMax - xMin;
  const lastBucket = buckets - 1;

  let dataEmitted = false;
  let lastIsBreak = false;

  // The open run: one bucket of one segment.
  let open = false;
  let runBucket = 0;
  let minIdx = 0;
  let maxIdx = 0;

  const flush = (): void => {
    if (!open) return;
    open = false;
    if (minIdx === maxIdx) {
      outX.push(x[minIdx]);
      outY.push(y[minIdx]);
    } else {
      const first = minIdx < maxIdx ? minIdx : maxIdx;
      const second = minIdx < maxIdx ? maxIdx : minIdx;
      outX.push(x[first], x[second]);
      outY.push(y[first], y[second]);
    }
    dataEmitted = true;
    lastIsBreak = false;
  };

  const emitBreak = (at: number): void => {
    if (!dataEmitted || lastIsBreak) return;
    // Without an x of its own, a break sits on the last emitted x (non-decreasing).
    if (at !== at) at = outX[outX.length - 1];
    outX.push(at);
    outY.push(null);
    lastIsBreak = true;
  };

  let gapAt = 0;
  let nextGap = gaps.length > 0 ? gaps[0] : -1;

  for (let i = 0; i < n; i++) {
    if (i === nextGap) {
      flush();
      emitBreak((x[i - 1] + x[i]) / 2);
      gapAt++;
      nextGap = gapAt < gaps.length ? gaps[gapAt] : -1;
    }
    const v = y[i];
    const xi = x[i];
    if (v !== v || xi !== xi) {
      flush();
      emitBreak(xi);
      continue;
    }
    let b = span > 0 ? Math.floor(((x[i] - xMin) / span) * buckets) : 0;
    if (b > lastBucket) b = lastBucket;
    else if (b < 0) b = 0;

    if (open && b === runBucket) {
      if (v < y[minIdx]) minIdx = i;
      if (v > y[maxIdx]) maxIdx = i;
    } else {
      flush();
      open = true;
      runBucket = b;
      minIdx = i;
      maxIdx = i;
    }
  }
  flush();

  if (lastIsBreak) {
    outX.pop();
    outY.pop();
  }
  return { x: outX, y: outY };
}

/** Pixel columns of a plot: Math.round(plotWidthCss × devicePixelRatio); 0 if the plot has no width. */
export function bucketCount(plotWidthCss: number, devicePixelRatio: number): number {
  if (!(plotWidthCss > 0) || !(devicePixelRatio > 0)) return 0;
  return Math.round(plotWidthCss * devicePixelRatio);
}
