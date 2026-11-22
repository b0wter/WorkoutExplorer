// The sample view models (F-13 design D-3): which synthetic golden data file, with which fixed
// settings, makes which committed sample in src/server/testdata/page/. Test support only; never
// imported by app code.

import { decodeDataFile, type WorkoutData } from '../datafile';
import { loadAddedLaps, loadGolden } from '../../../test-support/golden.mjs';
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
  /** F-45: the server's suggested title. */
  readonly suggestedTitle?: string;
  /** F-58: the trim string. */
  readonly trim?: string;
  /** F-56: the pins file and its count. */
  readonly pinsFile?: string;
  readonly pinCount?: number;
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

/** F-42 MS-3, F-64 LL-2: T2 has no lap, so its block is its heading alone. */
export function legWithoutLaps(data: WorkoutData): WorkoutData {
  return { ...data, meta: { ...data.meta, laps: data.meta.laps.filter((_, i) => i !== 3) } };
}

/** F-64 AC 7: run, ride, run without transitions; the swim becomes a run, T1 and T2 join the leg before them. */
export function runRideRun(data: WorkoutData): WorkoutData {
  const s = data.meta.sessions;
  const sessions = [{ ...s[0], sport: 'running', sub_sport: 'generic', end: s[1].end, end_index: s[1].end_index }, { ...s[2], end: s[3].end, end_index: s[3].end_index }, s[4]];
  return { ...data, meta: { ...data.meta, sessions } };
}

/** QA-46-r1-2 LS-2: lap triggers on a multisport file, so the Trigger column shows per leg table (F-64 LL-2). */
export function withTriggers(data: WorkoutData): WorkoutData {
  const laps = data.meta.laps.map((l, i) => (i === 0 ? { ...l, trigger: 'manual' } : i === 1 ? { ...l, trigger: 'distance' } : l));
  return { ...data, meta: { ...data.meta, laps } };
}

/** F-65 AL-6: the laps of added-laps.json's "device-laps" case, as the Go build makes them (Go: addedLaps). */
export function addedLaps(data: WorkoutData): WorkoutData {
  const c = loadAddedLaps()[0];
  if (c.name !== 'device-laps') throw new Error('added-laps.json: no device-laps case first');
  return { ...data, meta: { ...data.meta, laps: c.laps as WorkoutData['meta']['laps'] } };
}

/** F-44 PS-5: a pool swim whose session recorded no lengths count: "Pool" only. */
export function withoutLengths(data: WorkoutData): WorkoutData {
  const summary = { ...data.meta.summary };
  delete summary.lengths;
  return { ...data, meta: { ...data.meta, summary } };
}

/** QA-43-r1-3: markup in a recorded device name; the renderers print it as text. */
export function hostileDevice(data: WorkoutData): WorkoutData {
  return { ...data, meta: { ...data.meta, device: { ...data.meta.device, name: '<i>Dev</i> &amp; "x"' } } };
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
  { name: 'ride-suggested', golden: 'ride-30min', title: '', description: '', suggestedTitle: 'Morning ride near Köln' },
  { name: 'ride-trimmed', golden: 'ride-30min', title: '', description: '', trim: 'both' },
  { name: 'ride-pins', golden: 'ride-30min', title: '', description: '', pinsFile: 'pins.0123456789abcdef.json', pinCount: 3 },
  { name: 'triathlon', golden: 'triathlon', title: '', description: '' },
  { name: 'triathlon-lap-crossing', golden: 'triathlon', title: '', description: '', transform: lapCrossing },
  { name: 'triathlon-leg-without-laps', golden: 'triathlon', title: '', description: '', transform: legWithoutLaps },
  { name: 'triathlon-triggers', golden: 'triathlon', title: '', description: '', transform: withTriggers },
  { name: 'triathlon-cut', golden: 'triathlon-cut-90', title: '', description: '' },
  { name: 'run-ride-run', golden: 'triathlon', title: '', description: '', transform: runRideRun },
  { name: 'two-rides', golden: 'chained-ride-ride', title: '', description: '' },
  { name: 'pool-swim', golden: 'pool-swim', title: '', description: '' },
  { name: 'pool-swim-25yd', golden: 'pool-swim-25yd', title: '', description: '' },
  { name: 'pool-swim-no-lengths', golden: 'pool-swim', title: '', description: '', transform: withoutLengths },
  { name: 'ride-no-laps', golden: 'ride-no-laps', title: '', description: '' },
  { name: 'ride-lap-triggers', golden: 'ride-lap-triggers', title: '', description: '' },
  { name: 'ride-added-laps', golden: 'ride-lap-triggers', title: '', description: '', transform: addedLaps }, // F-65: the Added trigger
  { name: 'pool-swim-33m', golden: 'pool-swim-33m', title: '', description: '' },
  { name: 'pool-swim-mixed', golden: 'pool-swim-mixed', title: '', description: '' },
  // F-49: zones are client-owned (ZN-8), so the view model carries none.
  { name: 'zones-ride', golden: 'zones-wahoo-ride', title: '', description: '' },
  {
    name: 'hostile-text',
    golden: 'ride-30min',
    title: '<b>Bold</b> & "quoted" <script>alert(1)</script>',
    description: '<script>alert(2)</script>\n<img src=x onerror=alert(3)> &amp; done',
    transform: hostileDevice,
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
    suggestedTitle: spec.suggestedTitle,
    trim: spec.trim,
    pinsFile: spec.pinsFile,
    pinCount: spec.pinCount,
  });
  return { ...vm, Card: SAMPLE_CARD };
}

/** The committed form: pretty JSON, trailing newline. */
export function sampleText(model: PageViewModel): string {
  return JSON.stringify(model, null, 2) + '\n';
}
