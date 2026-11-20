// App entry (F-13 design D-16): the activity page. The server wrote the markup; this adopts it.
// On edit.html (marker #wx-edit) it loads the owner chunk instead (F-19 design E-1); on
// my-workouts.html (#wx-mine) the list page (F-53 MW-1). The viewing page loads neither.
import './page/page.css';
import { startPage } from './page/start';

const host = document.querySelector<HTMLElement>('#wx-edit,#wx-mine');
const failed = () => (host!.textContent = 'The page could not be loaded. Reload the page.');
if (host?.id == 'wx-mine') void import('./mine').then((m) => m.startMine(host), failed);
else if (host) void import('./edit/start.svelte').then((m) => m.startEdit(host), failed);
else void startPage(document);
