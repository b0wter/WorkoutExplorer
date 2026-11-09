// The range link (F-40 SR-1 to SR-3; AP-19, ADR-020 item 11): `#range={from}-{to}`, whole-second
// time offsets. Pure: imports nothing, never touches location, the DOM or storage. Hostile input:
// one anchored match on the whole fragment, no decoding, no exception path.

const FRAGMENT = /^#range=(\d{1,7})-(\d{1,7})$/;

/** First index with time[i] > x (strict) or time[i] >= x; time strictly increasing. */
function bound(time: ArrayLike<number>, x: number, strict: boolean): number {
  let lo = 0;
  let hi = time.length;
  while (lo < hi) {
    const m = (lo + hi) >>> 1;
    if (strict ? time[m] <= x : time[m] < x) lo = m + 1;
    else hi = m;
  }
  return lo;
}

/** The samples a fragment (`location.hash`, with its `#`) selects, cut to the data, or null when it
 *  is not exactly a range link, `from ≥ to`, or fewer than two samples fall inside. */
export function parseRangeLink(hash: string, time: ArrayLike<number>): [number, number] | null {
  const m = FRAGMENT.exec(hash);
  if (!m) return null;
  const from = +m[1];
  const to = +m[2];
  if (from >= to) return null;
  const a = bound(time, from, false);
  const b = bound(time, to, true) - 1;
  return a < b ? [a, b] : null;
}

/** The fragment, without `#`, for samples a ≤ b: start rounded down, end rounded up. */
export function rangeFragment(time: ArrayLike<number>, a: number, b: number): string {
  return `range=${Math.floor(time[a])}-${Math.ceil(time[b])}`;
}
