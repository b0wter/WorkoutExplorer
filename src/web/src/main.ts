// App entry (F-13 design D-16): the activity page. The server wrote the markup; this adopts it.
// On edit.html (marker #wx-edit) it loads the owner chunk instead (F-19 design E-1).
import './page/page.css';
import { startPage } from './page/start';

const edit = document.getElementById('wx-edit');
if (edit) void import('./edit/start.svelte').then((m) => m.startEdit(edit), () => (edit.textContent = 'The page could not be loaded. Reload the page.'));
else void startPage(document);
