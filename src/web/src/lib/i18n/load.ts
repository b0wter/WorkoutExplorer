// The i18n loader and markup applier (F-54 LG-2 to LG-4): language files, the first pass under the
// head script's hold, a drop-down switch, `data-t` markup. Kept per entry like the core; the activity
// page imports it only for a language other than English or a switch.
import { groups, markupText, pageLang, use, type Lang } from '.';

type Win = { wxLang?: Promise<Response> };

/** One group's file for `l`, `{}` for English, a group without a file, or a failure (LG-3: no message). */
export async function fetchGroup(group: string, l: Lang, doc: Document): Promise<Record<string, unknown>> {
  const meta = doc.querySelector<HTMLMetaElement>(`meta[name="wx-lang-${group}"]`);
  const url = l == 'en' ? undefined : meta?.dataset[l];
  if (!url) return {};
  try {
    const win = (doc.defaultView ?? {}) as Win;
    // The head script's request for the held group (LG-2): used once, never a second download.
    const pre = meta!.hasAttribute('data-hold') && l == pageLang(doc) ? win.wxLang : undefined;
    delete win.wxLang;
    const r = await (pre ?? fetch(url));
    const j: unknown = r.ok ? await r.json() : null;
    return j && typeof j == 'object' ? (j as Record<string, unknown>) : {};
  } catch {
    return {};
  }
}

/** The language a pending switch goes to; a later switch wins. */
let target: Lang = 'en';
const set = (l: Lang, parts: Record<string, unknown>[], doc: Document): void => {
  target = l;
  use(l, Object.assign({}, ...parts), doc);
};

/** A drop-down choice: every loaded group in `l`, then the listeners. A failed file leaves its group English. */
export async function setLang(l: Lang, doc: Document = document): Promise<void> {
  target = l;
  const parts = await Promise.all(groups.map((g) => fetchGroup(g, l, doc)));
  if (target == l) set(l, parts, doc);
}

/**
 * The first language pass of a page view (LG-2): loads `entryGroups` in the page language together
 * with `ready`, then runs `apply` and returns true, only while the head script's hold is still set
 * (a page without a hold: within 500 ms). Otherwise the page view stays English. Never rejects.
 */
export async function startLang(entryGroups: readonly string[], doc: Document = document, ready?: Promise<unknown>, apply?: () => void): Promise<boolean> {
  const html = doc.documentElement;
  const l = pageLang(doc);
  groups.push(...entryGroups.filter((g) => !groups.includes(g)));
  if (l == 'en') return false;
  const held = 'hold' in html.dataset;
  let late = false;
  if (!held) setTimeout(() => (late = true), 500);
  try {
    const parts = await Promise.all(groups.map((g) => fetchGroup(g, l, doc)));
    await ready;
    if (held ? !('hold' in html.dataset) : late) return false;
    set(l, parts, doc);
    apply?.();
    return true;
  } catch {
    return false;
  } finally {
    delete html.dataset.hold;
  }
}

/** English of the markup keys, as first read from the markup (for a switch back to English). */
const markupEn: Record<string, string> = {};
const ATTRS = ['title', 'aria-label', 'placeholder', 'alt'];

/** A markup text: a translated string, else the markup's own English. */
function text(key: string, english: string): string {
  markupEn[key] ??= english;
  return markupText(key) ?? markupEn[key];
}

/**
 * Rewrites the markup's fixed texts, text only (LG-4): `data-t` on an element whose content is
 * text, where element children with `data-p` are kept and placed by their `{name}` placeholder;
 * `data-t-title`, `data-t-aria-label`, `data-t-placeholder`, `data-t-alt` for attributes.
 */
export function applyMarkup(root: ParentNode): void {
  for (const el of root.querySelectorAll<HTMLElement>('[data-t]')) {
    const kids = [...el.children] as HTMLElement[];
    const english = [...el.childNodes].map((n) => (n.nodeType == 1 ? `{${(n as HTMLElement).dataset.p}}` : n.textContent)).join('');
    const value = text(el.dataset.t!, english);
    if (!kids.length) el.textContent = value;
    else el.replaceChildren(...value.split(/\{(\w+)\}/).map((s, i) => (i % 2 ? (kids.find((k) => k.dataset.p == s) ?? '') : s)));
  }
  for (const a of ATTRS) {
    for (const el of root.querySelectorAll(`[data-t-${a}]`)) el.setAttribute(a, text(el.getAttribute(`data-t-${a}`)!, el.getAttribute(a) ?? ''));
  }
}

