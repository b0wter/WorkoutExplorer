// The sample view models (F-13 design D-3): which synthetic golden data file, with which fixed
// settings, makes which committed sample in src/server/testdata/page/. Test support only; never
// imported by app code.

import { decodeDataFile, type WorkoutData } from '../datafile';
import { loadGolden } from '../../../test-support/golden.mjs';
import { fileHash } from '../../../test-support/samples.mjs';
import { buildPageViewModel } from './view';
import type { PageViewModel } from './model';

export interface SampleSpec {
  readonly name: string;
  readonly golden: string;
  readonly title: string;
  readonly description: string;
  /** F-16: the uploader hid the entire route. */
  readonly routeHidden?: boolean;
  /** Derives a file the corpus has no instance of (a workout without heart rate). */
  readonly transform?: (data: WorkoutData) => WorkoutData;
}

/** The same workout without any heart-rate value: stream, summary and lap totals. */
export function withoutHeartRate(data: WorkoutData): WorkoutData {
  const strip = <T extends object>(t: T): T => {
    const out: Record<string, unknown> = { ...(t as Record<string, unknown>) };
    delete out.avg_heart_rate;
    delete out.max_heart_rate;
    return out as T;
  };
  const metrics = { ...data.metrics };
  delete metrics.heart_rate;
  return {
    ...data,
    metrics,
    meta: {
      ...data.meta,
      summary: strip(data.meta.summary),
      sessions: data.meta.sessions.map((s) => ({ ...s, totals: strip(s.totals) })),
      laps: data.meta.laps.map((l) => ({ ...l, totals: strip(l.totals) })),
    },
  };
}

/** F-42 MS-3: the swim lap runs 30 s into T1, T1's lap starts there: the crossing lap stays under Swim. */
export function lapCrossing(data: WorkoutData): WorkoutData {
  const laps = data.meta.laps.map((l, i) =>
    i === 0 ? { ...l, end: 629, end_index: 630 } : i === 1 ? { ...l, start: 630, start_index: 630 } : l,
  );
  return { ...data, meta: { ...data.meta, laps } };
}

/** F-42 MS-3: T2 has no lap, so its separator row stands alone before Run's. */
export function legWithoutLaps(data: WorkoutData): WorkoutData {
  return { ...data, meta: { ...data.meta, laps: data.meta.laps.filter((_, i) => i !== 3) } };
}

export const EXPIRES_AT = '2026-10-30T09:15:00Z';

/** F-23 PC-6: the Open Graph card is built in Go only, so every sample carries this placeholder for the
 *  template; the Go sample comparison skips it. */
export const SAMPLE_CARD = { Title: 'Sample', Description: 'Sample', Image: 'http://127.0.0.1:8088/preview.png' } as const;

export const SAMPLES: readonly SampleSpec[] = [
  { name: 'ride-30min', golden: 'ride-30min', title: 'Morning loop', description: 'Easy spin.\nSecond line.' },
  { name: 'ride-12h', golden: 'ride-12h', title: '', description: '' },
  { name: 'run-30min', golden: 'run-30min', title: '', description: '' },
  { name: 'ride-no-hr', golden: 'ride-30min', title: '', description: '', transform: withoutHeartRate },
  { name: 'indoor-ride', golden: 'indoor-no-gps', title: 'Trainer', description: '' },
  { name: 'indoor-unknown-zone', golden: 'indoor-no-distance-no-gps', title: '', description: '' },
  { name: 'route-hidden', golden: 'indoor-no-gps', title: '', description: '', routeHidden: true },
  { name: 'triathlon', golden: 'triathlon', title: '', description: '' },
  { name: 'triathlon-lap-crossing', golden: 'triathlon', title: '', description: '', transform: lapCrossing },
  { name: 'triathlon-leg-without-laps', golden: 'triathlon', title: '', description: '', transform: legWithoutLaps },
  { name: 'triathlon-cut', golden: 'triathlon-cut-90', title: '', description: '' },
  { name: 'two-rides', golden: 'chained-ride-ride', title: '', description: '' },
  { name: 'pool-swim', golden: 'pool-swim', title: '', description: '' },
  { name: 'ride-no-laps', golden: 'ride-no-laps', title: '', description: '' },
  {
    name: 'hostile-text',
    golden: 'ride-30min',
    title: '<b>Bold</b> & "quoted" <script>alert(1)</script>',
    description: '<script>alert(2)</script>\n<img src=x onerror=alert(3)> &amp; done',
  },
];

/** Builds one sample from its golden file. */
export function buildSample(spec: SampleSpec): PageViewModel & { readonly Card: typeof SAMPLE_CARD } {
  const golden = loadGolden().find((g) => g.name === spec.golden);
  if (!golden) throw new Error(`no golden ${spec.golden}`);
  const decoded = decodeDataFile(golden.bin);
  if (!decoded.ok) throw new Error(decoded.error.message);
  const data = spec.transform ? spec.transform(decoded.data) : decoded.data;
  const vm = buildPageViewModel(data, {
    title: spec.title,
    description: spec.description,
    expiresAt: EXPIRES_AT,
    publicId: spec.name,
    dataFile: `data.${fileHash(golden.bin)}.bin`,
    routeHidden: spec.routeHidden,
  });
  return { ...vm, Card: SAMPLE_CARD };
}

/** The committed form: pretty JSON, trailing newline. */
export function sampleText(model: PageViewModel): string {
  return JSON.stringify(model, null, 2) + '\n';
}
