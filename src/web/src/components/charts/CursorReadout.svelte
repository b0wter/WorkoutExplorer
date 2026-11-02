<script lang="ts">
  // Values at the cursor, one row above the charts (F-05 design, "Readout"; AP-6).
  // Cells are rendered once; the cursor listener assigns textContent directly
  // (no Svelte reactivity on the hot path, D-10). Values come from the
  // full-resolution arrays (D-9) through readoutFields, which decides which
  // fields exist. No aria-live: it would be announced up to 60 times a second.
  import { onMount } from 'svelte';
  import type { WorkoutData } from '../../lib/datafile';
  import type { CursorStore } from '../../lib/cursor';
  import type { Units } from '../../lib/format';
  import { MISSING_TEXT, readoutFields } from '../../lib/charts';

  interface Props {
    data: WorkoutData;
    cursor: CursorStore;
    /** Fixed for the life of this instance (ChartStack keys on it). */
    units?: Units;
  }

  let { data, cursor, units = 'metric' }: Props = $props();

  const fields = $derived(readoutFields(data, units));
  let row: HTMLDivElement;

  onMount(() => {
    // Fixed for the life of this instance (ChartStack keys on data).
    const f = fields;
    // The value cells, in field order; collected once (no reactive array on the hot path).
    const els = row.querySelectorAll<HTMLSpanElement>('.value');
    const c = cursor;
    return c.subscribe((i) => {
      for (let k = 0; k < f.length; k++) els[k].textContent = i === null ? MISSING_TEXT : f[k].text(i);
    });
  });
</script>

<!-- The cell look (label line over value line, card) is shared with the range panel (page.css,
     "Value cards"); this component keeps only its own flow and sticking. Values are
     left-aligned like the panel's; the unit may shift when the digit count changes (user 29 Oct 2026). -->
<div class="readout" bind:this={row}>
  {#each fields as field (field.key)}
    <div class="cell">
      <div class="label">{field.label}</div>
      <div class="val"><span class="value">{MISSING_TEXT}</span>{#if field.unit}&nbsp;<span class="unit">{field.unit}</span>{/if}</div>
    </div>
  {/each}
</div>

<style>
  /* Normal flow, wraps between cells, auto height, sticky above the charts (D-17, v1.4). The page
     (page.css) makes it a card with a fixed grid, so its rows are reserved (F-13 D-10). */
  .readout {
    display: flex;
    flex-wrap: wrap;
    column-gap: 16px;
    align-items: start;
    position: sticky;
    top: 0;
    z-index: 1;
    background: var(--bg, #fff);
    padding: 0 16px 0 var(--y-axis-width);
    contain: layout;
  }
  .cell {
    flex: none;
    white-space: nowrap;
  }
</style>
