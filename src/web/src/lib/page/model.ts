// The page view model (F-13 design D-2, ADR-026 item 2): the only input of the activity page's
// template and of the edit page's static components. Every value is a display-ready string.
// Keys are capitalised because Go's html/template reads exported fields (and map keys) by that
// name; the committed samples in src/server/testdata/page/ are the contract with F-15's Go builder.
// A group that does not apply is null: the template only tests it, never prints it.

import type { LapsSection } from '../sections';

export interface PageGroup {
  /** The document title: the workout title, or the default ("Ride, 9 Oct 2026"). */
  readonly Title: string;
  /** Root-relative viewing path, '/w/{public_id}'. The copy link target (SH-6). */
  readonly ViewPath: string;
  /** Root-relative data file URL, '/w/{public_id}/data.{hash}.bin'. Never built by the client. */
  readonly DataURL: string;
}

export interface HeaderGroup {
  /** Title shown in the header (same as Page.Title). */
  readonly Title: string;
  readonly SportName: string;
  /** Icon key: a sport or 'generic'. */
  readonly SportIcon: string;
  /** '9 Oct 2026, 07:30' in the zone at the start position; ' UTC' appended when the zone is unknown. */
  readonly Start: string;
  /** Plain text, '' when there is none. */
  readonly Description: string;
  /** UTC expiry time, RFC 3339: the browser computes "Expires in …" from it. */
  readonly ExpiresAt: string;
  /** The server's estimate, shown without JavaScript: 'Expires around 30 Oct 2026'. */
  readonly ExpiresText: string;
}

export interface SummaryItem {
  readonly Key: string;
  readonly Label: string;
  readonly Value: string;
  readonly Unit: string;
  /** The maximum as a second value in the same cell; '' when there is none. */
  readonly Max: string;
}

export interface LapsGroup {
  /** Empty when None is set. */
  readonly Columns: readonly { readonly Key: string; readonly Label: string }[];
  /** One array of cell strings per lap, one cell per column ('–' where a lap has no value). */
  readonly Rows: readonly (readonly string[])[];
  /** 'yes': the file has no laps but its distance advances, so the module shows "No laps recorded."
   *  instead of the table (F-18 L-8); '' otherwise. */
  readonly None: string;
}

export interface PageViewModel {
  readonly Page: PageGroup;
  readonly Header: HeaderGroup;
  /** AP-2 order; metrics the workout lacks are absent. */
  readonly Summary: readonly SummaryItem[];
  /** F-43 DI-7: 'Recorded with {name} · {sensors}', 'Recorded with {name}', 'Sensors: {sensors}' or ''
   *  (no line). Plain text. */
  readonly Device: string;
  /** AP-3, as strings the template only tests: Map 'yes' reserves the map box, NoLocation 'yes' shows the hint, Hidden 'yes' shows the fully-hidden notice (F-16 FE-1). At most one is set. */
  readonly Route: { readonly Map: string; readonly NoLocation: string; readonly Hidden: string };
  /** Reserved-size classes of the chart slot: 'charts-4 cells-6' (D-10), plus ' has-distance' with a
   *  distance stream (F-17 R-8), or '' with no charts. */
  readonly ChartsClass: string;
  /** Null when the file has neither laps nor an advancing distance (F-18 L-8). */
  readonly Laps: PageLaps | null;
}

/** The laps group of the page view model. Sections (F-42 MS-3, MS-6): Rows grouped under their
 *  separator rows, every section in order; empty without sections. When set, the template prints
 *  them instead of Rows. */
export interface PageLaps extends LapsGroup {
  readonly Sections: readonly LapsSection[];
}

/** What the builder needs besides the decoded data file: the settings of the workout. */
export interface PageSettings {
  readonly title: string;
  readonly description: string;
  /** RFC 3339 UTC. */
  readonly expiresAt: string;
  readonly publicId: string;
  /** Hashed file name, 'data.{hash}.bin'. */
  readonly dataFile: string;
  /** F-16 H-5: the uploader hid the entire route; not derivable from the data file. */
  readonly routeHidden?: boolean;
}
