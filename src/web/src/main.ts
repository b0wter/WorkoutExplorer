// App entry (F-13 design D-16): the activity page. The server wrote the markup; this adopts it.
// On edit.html (marker #wx-edit) it loads the owner chunk instead (F-19 design E-1); on
// my-workouts.html (#wx-mine) the list page (F-53 MW-1). The viewing page loads neither.
// F-54 LG-3: those pages' language files load together with their chunk, within 500 ms (no hold).
import './page/page.css';
import { startPage } from './page/start';
import { loadGroup, pageLang } from './lib/i18n';
import { t } from './page/texts';

const host = document.querySelector<HTMLElement>('#wx-edit,#wx-mine');
const failed = () => (host!.textContent = t('pageFailed'));
const lang = (groups: string[]): Promise<unknown> =>
  pageLang() === 'en' ? Promise.all(groups.map((g) => loadGroup(g))) : import('./lib/i18n/load').then((m) => m.startLang(groups, document));
const lazy = <M>(groups: string[], chunk: Promise<M>): Promise<M> => Promise.all([chunk, lang(groups)]).then(([m]) => m);
if (host?.id == 'wx-mine') void lazy(['mine'], import('./mine')).then((m) => m.startMine(host), failed);
else if (host) void lazy(['page', 'owner'], import('./edit/start.svelte')).then((m) => m.startEdit(host), failed);
else void startPage(document);
