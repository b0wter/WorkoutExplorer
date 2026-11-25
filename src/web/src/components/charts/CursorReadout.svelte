<script lang="ts">
  // Values at the cursor, one row above the charts (F-05 design, "Readout"; AP-6).
  // Cells are rendered once; the cursor listener assigns textContent directly
  // (no Svelte reactivity on the hot path, D-10). Values come from the
  // full-resolution arrays (D-9) through readoutFields, which decides which
  // fields exist. No aria-live: it would be announced up to 60 times a second.
  import { onMount, untrack } from 'svelte';
  import type { WorkoutData } from '../../lib/datafile';
  import type { CursorStore } from '../../lib/cursor';
  import type { Units } from '../../lib/format';
  import { MISSING_TEXT, readoutFields } from '../../lib/charts';
  import { ordered, sportIn } from '../../lib/sections/order';

  interface Props {
    data: WorkoutData;
    cursor: CursorStore;
    /** Fixed for the life of this instance (ChartStack keys on it). */
    units?: Units;
    sport?: string;
  }

  let { data, cursor, units = 'metric', sport }: Props = $props();

  // F-67 LP-10: on a multisport workout the fields read in the sport of the leg under the cursor
  // (speed or pace), '–' for speed in a transition; the labels re-render only when that sport changes.
  // F-72 YD-2: the distance unit follows the view's sport, never the section under the cursor.
  const dist = untrack(() => sport ?? data.meta.sport);
  const fields = $derived(readoutFields(data, units, sport, dist));
  let row: HTMLDivElement;

  onMount(() => {
    // Fixed for the life of this instance (ChartStack keys on data), but for a leg's sport (LP-10).
    let f = fields;
    const secs = ordered(data.meta);
    // The value cells, in field order; collected once (no reactive array on the hot path).
    const els = row.querySelectorAll<HTMLSpanElement>('.value');
    const c = cursor;
    return c.subscribe((i) => {
      const s = secs && i !== null ? sportIn(secs, i) : sport;
      if (s !== sport) f = readoutFields(data, units, (sport = s), dist);
      for (let k = 0; k < f.length; k++) els[k].textContent = i === null || (s === 'transition' && f[k].key === 'speed') ? MISSING_TEXT : f[k].text(i);
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
