import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

export default {
  preprocess: vitePreprocess(),
  // No `window.__svelte` version marker in the bundle (ADR-011 C3).
  compilerOptions: { discloseVersion: false },
};
