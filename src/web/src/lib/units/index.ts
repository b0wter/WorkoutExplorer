// The units store (F-14 design D-3, D-4; ADR-027 item 4): the viewer's unit system for one page.
// Framework-free; memory only (no cookie, storage, URL or history state). One instance per page,
// created by the page glue and passed to every consumer. The only module that reads the browser
// language for units. Imports only the Units type.

import type { Units } from '../format';

export type UnitsListener = (units: Units) => void;

export interface UnitsStore {
  get(): Units;
  /** Delivers synchronously to every listener when the value changed; a throwing listener is reported and skipped. */
  set(units: Units): void;
  /** Does not call the listener on subscribe. Returns the unsubscribe function. */
  subscribe(listener: UnitsListener): () => void;
}

export function createUnitsStore(initial: Units, report: (error: unknown) => void = console.error): UnitsStore {
  let value = initial;
  let listeners: readonly UnitsListener[] = [];
  return {
    get: () => value,
    set(units) {
      if (units === value) return;
      value = units;
      for (const l of listeners) {
        try {
          l(units);
        } catch (error) {
          report(error);
        }
      }
    },
    subscribe(listener) {
      listeners = [...listeners, listener];
      return () => {
        listeners = listeners.filter((l) => l !== listener);
      };
    },
  };
}

/** Imperial for a US-region tag (the first language, `en-US`, `es_US`, `en-Latn-US`); metric for any other
 *  region, no region (`en`) or an unparseable tag (question 8). By hand, no Intl (engines differ). */
export function defaultUnits(languages: readonly string[] | undefined, language?: string): Units {
  const tag = languages?.[0] ?? language ?? '';
  const parts = tag.split(/[-_]/);
  for (let i = 1; i < parts.length; i++) {
    // Region: two letters (a script is four letters, a variant five or more).
    if (/^[A-Za-z]{2}$/.test(parts[i])) return parts[i].toUpperCase() === 'US' ? 'imperial' : 'metric';
  }
  return 'metric';
}
