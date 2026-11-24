// Text groups and language files (F-54 LG-3). Read by the build (vite.config.ts, `wx-lang-files`)
// and the completeness tests; never by the bundle. German, Spanish and French live in ./de.json,
// ./es.json and ./fr.json, one object per table. A table is one English source: a `texts.ts`
// module (`export const en`; the table is its folder's path under src/) or the `data-t` keys of
// the markup files below that no code table holds (a key in both must have the same English). A group's language file is the union of its tables; a lazy chunk's group is
// requested together with its import(). Tiny lazy groups joined their entry's group (LG-3).

/** Tables of the markup: `data-t` keys whose English is the markup's (LG-4). */
export const MARKUP: Readonly<Record<string, readonly string[]>> = {
  'markup/view': ['index.html'],
  'markup/upload': ['upload.html'],
  'markup/static': ['about.html', 'generic.html'],
};

export const GROUPS: Readonly<Record<string, readonly string[]>> = {
  // The viewing page's entry, with the lazy map, sections, splits, zones, efforts and pins viewer,
  // and the page components the edit page renders.
  page: ['markup/view', 'page', 'components/page', 'lib/format', 'lib/format/dates-texts', 'lib/page', 'lib/page/device-texts', 'lib/charts', 'components/map', 'components/map/map-texts', 'sections', 'splits', 'lib/splits', 'zones', 'efforts', 'pins', 'lib/pins'],
  help: ['howto', 'summary-info', 'dialog'],
  report: ['report', 'lib/errors'],
  owner: ['edit', 'lib/errors'],
  mine: ['mine', 'lib/format/dates-texts'],
  upload: ['markup/upload', 'upload', 'lib/upload', 'lib/errors'],
  static: ['markup/static'],
};

/** Per HTML entry: the groups it names in its head, the first held by the head script when `hold`. */
export const PAGES: Readonly<Record<string, { groups: readonly string[]; hold: boolean }>> = {
  'index.html': { groups: ['page', 'help', 'report'], hold: true },
  'edit.html': { groups: ['page', 'owner', 'help'], hold: false },
  'my-workouts.html': { groups: ['mine'], hold: false },
  'upload.html': { groups: ['upload'], hold: true },
  'about.html': { groups: ['static'], hold: true },
  'generic.html': { groups: ['static'], hold: true },
};

export const TRANSLATED = ['de', 'es', 'fr'] as const;
