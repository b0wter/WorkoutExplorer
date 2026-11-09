// The delete link's confirmation page (F-37 DK-11; ADR-024: plain TypeScript, no Svelte).
import './report-delete.css';
import { start } from './page';

start(document, window.location.hash, (input, init) => window.fetch(input, init));
