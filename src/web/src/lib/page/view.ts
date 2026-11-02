// The page view model as a whole (F-13 design D-2 to D-7, ADR-026 items 2, 3). Pure. A file of
// its own, apart from the summary and laps builders the viewing page's enhancers share, so that
// only the edit page's owner chunk carries it (F-19, bundle budget). The Go builder (F-15) must
// reproduce the committed samples exactly.

import type { WorkoutData } from '../datafile';
import { presentCharts } from '../charts';
import { routePresence } from '../route/presence';
import { formatDate, formatLocalStart, sportIcon, sportName } from '../format/dates';
import { distanceAdvances, lapsGroup, summaryItems } from './build';
import type { LapsGroup, PageSettings, PageViewModel } from './model';

/** The laps module of a file without laps but with an advancing distance (F-18 L-8). */
const NO_LAPS: LapsGroup = { Columns: [], Rows: [], None: 'yes' };

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
  return 'Expires around ' + formatDate(expiresAt, offsetSeconds);
}

export function buildPageViewModel(data: WorkoutData, settings: PageSettings): PageViewModel {
  const meta = data.meta;
  const zone = meta.time_zone;
  const offset = zone.known ? zone.offset_seconds : 0;
  const title = settings.title.trim() === '' ? defaultTitle(meta.sport, meta.start_time, offset) : settings.title;
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
    Route: {
      Map: presence === 'route' ? 'yes' : '',
      NoLocation: presence === 'no-location-data' && !settings.routeHidden ? 'yes' : '',
      Hidden: presence === 'no-location-data' && settings.routeHidden ? 'yes' : '',
    },
    ChartsClass: chartsClass(data),
    Laps: lapsGroup(meta.sport, meta.laps) ?? (distanceAdvances(data) ? NO_LAPS : null),
  };
}
