// The My workouts page (F-53 MW-4 to MW-10). LAZY: reached only through main.ts's import() on
// my-workouts.html (#wx-mine). Sends no request (AC 47). CRITICAL (MW-5): every stored string
// reaches the DOM as text only (textContent), and the only attribute built from storage is an
// href from lib/mine's checked, root-relative paths; same tab, no target.
// The stylesheet by URL, not as a side-effect import: that would add one more file name to the
// activity entry's preload map, over its 60-byte ceiling (F-53 Cost). Same origin (CSP style-src 'self').
import css from './mine.css?url';
import { clear, forget, prune, type Row, type StoreGetter } from '../lib/mine';
import { formatDate, sportName } from '../lib/format/dates';
import { MISSING_TEXT } from '../lib/format';
import { impressum } from '../lib/page/impressum';
import { lang } from '../lib/i18n';
import { t } from './texts';

/** 'in 3 days' (MW-9): minutes below an hour, hours below two days, else days. */
export function relative(ms: number, rtf = new Intl.RelativeTimeFormat(lang())): string {
  const min = ms / 60_000;
  if (min < 60) return rtf.format(Math.max(1, Math.round(min)), 'minute');
  if (min < 48 * 60) return rtf.format(Math.round(min / 60), 'hour');
  return rtf.format(Math.round(min / 1440), 'day');
}

const day = (date: string): string => formatDate(date + 'T00:00:00Z', 0);

/** UP-5: the stored title (owner's or suggested), else the default title built here. */
export const rowTitle = (r: Row): string => (r.title?.trim() ? r.title : sportName(r.sport ?? '') + (r.date ? ', ' + day(r.date) : ''));

/** Renders once the stylesheet has loaded or failed, so the table never shows unstyled. */
export function startMine(host: HTMLElement, store?: StoreGetter, now?: () => number): void {
  const link = host.ownerDocument.createElement('link');
  link.rel = 'stylesheet';
  link.href = css;
  link.onload = link.onerror = () => show(host, store, now);
  host.ownerDocument.head.append(link);
}

export function show(host: HTMLElement, store: StoreGetter = () => window.localStorage, now: () => number = Date.now): void {
  const doc = host.ownerDocument;
  doc.title = t('mineTitle') + ' · Workout Explorer';
  const el = <K extends keyof HTMLElementTagNameMap>(tag: K, text = '', parent?: HTMLElement): HTMLElementTagNameMap[K] => {
    const e = doc.createElement(tag);
    e.textContent = text;
    parent?.append(e);
    return e;
  };
  const button = (text: string, act: string, parent: HTMLElement): void => {
    const b = el('button', text, parent);
    b.type = 'button';
    b.className = 'mine-button';
    b.dataset.act = act;
  };
  const link = (text: string, href: string, parent: HTMLElement): void => {
    el('a', text, parent).href = href;
  };
  const main = el('main');
  main.className = 'mine';
  /** The rows on screen, by their row index (Forget). */
  let rows: Row[] = [];

  /** `cleared`: the key was just removed; shown empty without writing it back (OQ-279). */
  const render = (mode?: 'asking' | 'cleared', focus?: string): void => {
    main.replaceChildren();
    el('h1', t('mineTitle'), main).tabIndex = -1;
    el('p', t('mineNote'), main);
    const got = mode === 'cleared' ? [] : prune(store, now());
    rows = got ?? [];
    if (!got) el('p', t('mineUnavailable'), main);
    else if (got.length === 0) el('p', t('mineEmpty'), main);
    else {
      const box = el('div', '', main);
      box.className = 'mine-box';
      const table = el('table', '', box);
      const head = el('tr', '', el('thead', '', table));
      for (const h of ['colWorkout', 'colSport', 'colDate', 'colStatus', 'colExpires', 'colLinks'] as const) el('th', t(h), head);
      el('th', '', head);
      const body = el('tbody', '', table);
      const at = now();
      got.forEach((r, i) => {
        const tr = el('tr', '', body);
        el('td', rowTitle(r), tr);
        el('td', r.sport === undefined ? MISSING_TEXT : sportName(r.sport), tr);
        el('td', r.date ? day(r.date) : MISSING_TEXT, tr);
        el('td', r.published ? t('statusPublished') : t('statusDraft'), tr);
        el('td', relative(r.expiresMs - at), tr);
        const links = el('td', '', tr);
        links.className = 'mine-links';
        if (r.edit) link(t('linkEdit'), r.edit, links);
        if (r.view) link(t('linkView'), r.view, links);
        button(t('forget'), 'forget:' + i, el('td', '', tr));
      });
      const p = el('p', '', main);
      if (mode !== 'asking') button(t('clearList'), 'clear', p);
      else {
        p.className = 'mine-confirm';
        el('span', t('clearAsk'), p);
        el('br', '', p);
        button(t('removeAll'), 'confirm', p);
        p.append(' ');
        button(t('cancelClear'), 'cancel', p);
      }
    }
    link(t('mineUpload'), '/', el('p', '', main));
    // QA-53-r1-4: keep the place; the row's slot, else the heading.
    (main.querySelector<HTMLElement>(`[data-act="${focus}"]`) ?? main.querySelector<HTMLElement>('h1'))?.focus();
  };
  host.replaceChildren(main);
  render();

  main.addEventListener('click', (e) => {
    const act = (e.target as Element).closest<HTMLElement>('[data-act]')?.dataset.act;
    if (!act) return;
    if (act.startsWith('forget:')) {
      const r = rows[Number(act.slice(7))];
      if (r) forget(store, r.raw);
      render(undefined, act); // the next row now holds this index
    } else if (act === 'clear') render('asking', 'cancel');
    else if (act === 'confirm') render(clear(store) ? 'cleared' : undefined);
    else if (act === 'cancel') render(undefined, 'clear');
  });

  const foot = el('footer', '', host);
  foot.className = 'mine-foot';
  link(t('mineAbout'), '/about', foot);
  impressum(foot);
}
