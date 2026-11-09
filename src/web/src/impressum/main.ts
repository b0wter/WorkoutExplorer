// The Impressum page's entry (F-38 IM-2; ADR-024: plain TypeScript, no Svelte).
import './impressum.css';
import { start } from './page';

start(document, (input, init) => window.fetch(input, init));
