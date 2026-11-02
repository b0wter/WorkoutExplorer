// Upload page entry (F-11 design U-1, U-2; ADR-024). Plain TypeScript: no Svelte.
import './upload.css';
import { start } from './page';

start(document, window, {
  fetch: (input, init) => window.fetch(input, init),
  xhr: () => new XMLHttpRequest(),
  navigate: (url) => window.location.assign(url),
  origin: window.location.origin,
  statusWaitMs: 4000,
});
