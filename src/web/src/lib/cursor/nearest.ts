// nearestIndex: maps a value on an axis (elapsed time now, distance in F-17)
// to the nearest sample index (F-05 design, "Cursor store"; review decision 5:
// tie → earliest sample). Binary search, no allocation.

/** values strictly increasing, length ≥ 1. Index of the value nearest to target; tie → lower index;
 *  target below the first / above the last → 0 / length−1; non-finite target → −1. */
export function nearestIndex(values: Float64Array, target: number): number {
  if (!Number.isFinite(target)) return -1;
  const last = values.length - 1;
  if (target <= values[0]) return 0;
  if (target >= values[last]) return last;
  // Invariant: values[lo] ≤ target < values[hi].
  let lo = 0;
  let hi = last;
  while (hi - lo > 1) {
    const mid = (lo + hi) >>> 1;
    if (values[mid] <= target) lo = mid;
    else hi = mid;
  }
  return target - values[lo] <= values[hi] - target ? lo : hi;
}
