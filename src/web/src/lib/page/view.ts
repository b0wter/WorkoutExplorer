// The page view model as a whole (F-13 design D-2 to D-7, ADR-026 items 2, 3). Pure. A file of
// its own, apart from the summary and laps builders the viewing page's enhancers share, so that
// only the edit page's owner chunk carries it (F-19, bundle budget). The Go builder (F-15) must
// reproduce the committed samples exactly.

import type { WorkoutData } from '../datafile';
import type { Units } from '../format';
import { presentCharts } from '../charts';
import { routePresence } from '../route/presence';
import { formatDate, formatLocalStart, sportIcon, sportName } from '../format/dates';
import { sections, lapsBySection } from '../sections';
import { distanceAdvances, lapsGroup, summaryItems } from './build';
import { deviceLine } from './device';
import { t } from './device-texts';
import type { PageLaps, PageLapsLeg, PageSettings, PageViewModel } from './model';

/** The hidden module of a file without laps but with an advancing distance (F-46 LS-3) or lengths (F-48 PL-7). */
const NO_LAPS: PageLaps = { Columns: [], Rows: [], None: 'yes', NotePace: '', NoteSpeed: '', Legs: [] };

/** F-64 LL-2: one block per section with its own laps table, built for the section's sport. */
function lapLegs(meta: WorkoutData['meta'], units?: Units): PageLapsLeg[] {
  const secs = sections(meta);
  return lapsBySection(secs, meta.laps).map((laps, k) => {
    const g = lapsGroup(secs![k].sport, laps, units);
    return { Name: secs![k].name, Sport: secs![k].sport, Columns: g?.Columns ?? [], Rows: g?.Rows ?? [], NotePace: g?.NotePace ?? '', NoteSpeed: g?.NoteSpeed ?? '' };
  });
}

/** The default title of an untitled workout (UP-4): "Ride, 9 Oct 2026". */
export function defaultTitle(sport: string, startTime: string, offsetSeconds: number): string {
  return sportName(sport) + ', ' + formatDate(startTime, offsetSeconds);
}

/** Readout cells: time, distance if present, then one per chart (F-05); 'has-distance' with a
 *  distance stream reserves the axis toolbar row (F-17 R-8). */
function chartsClass(data: WorkoutData): string {
  const n = presentCharts(data).length;
  if (n === 0) return '';
  const distance = data.metrics.distance !== undefined;
  const cells = 1 + (distance ? 1 : 0) + n;
  return 'charts-' + n + ' cells-' + cells + (distance ? ' has-distance' : '');
}

/** Display date of an instant in the workout's zone: for the expiry estimate (OQ-62). */
function expiryEstimate(expiresAt: string, offsetSeconds: number): string {
  return t('expiresAround', { date: formatDate(expiresAt, offsetSeconds) });
}

export function buildPageViewModel(data: WorkoutData, settings: PageSettings): PageViewModel {
  const meta = data.meta;
  const zone = meta.time_zone;
  const offset = zone.known ? zone.offset_seconds : 0;
  const title = settings.title.trim() === '' ? settings.suggestedTitle || defaultTitle(meta.sport, meta.start_time, offset) : settings.title;
  const base = '/w/' + settings.publicId;
  const presence = routePresence(data);
  return {
    Page: { Title: title, ViewPath: base, DataURL: base + '/' + settings.dataFile },
    Header: {
      Title: title,
      SportName: sportName(meta.sport),
      SportIcon: sportIcon(meta.sport),
      Start: formatLocalStart(meta.start_time, zone),
      Description: settings.description,
      ExpiresAt: settings.expiresAt,
      ExpiresText: expiryEstimate(settings.expiresAt, offset),
    },
    Summary: summaryItems(meta.sport, meta.summary),
    Trim: settings.trim ?? '',
    Device: deviceLine(meta.device),
    Route: {
      Map: presence === 'route' ? 'yes' : '',
      NoLocation: presence === 'no-location-data' && !settings.routeHidden ? 'yes' : '',
      Hidden: presence === 'no-location-data' && settings.routeHidden ? 'yes' : '',
      Gpx: presence === 'route' && meta.sport !== 'multisport' ? 'yes' : '',
    },
    Pins: settings.pinsFile && (settings.pinCount ?? 0) > 0 ? { URL: base + '/' + settings.pinsFile, Count: String(settings.pinCount) } : { URL: '', Count: '' },
    ChartsClass: chartsClass(data),
    Laps: pageLaps(data),
  };
}

/** The laps module (AP-8, F-46 LS-3, F-64 LL-2); the edit page builds it in its units (F-65 AL-8). */
export function pageLaps(data: WorkoutData, units?: Units): PageLaps | null {
  const meta = data.meta;
  const laps = lapsGroup(meta.sport, meta.laps, units);
  return laps ? { ...laps, Legs: lapLegs(meta, units) } : distanceAdvances(data) || meta.lengths.length > 0 ? NO_LAPS : null;
}
