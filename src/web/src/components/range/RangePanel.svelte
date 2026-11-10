<script lang="ts">
  // The range panel (F-17 design R-9, range values table; AP-7). Mounted by the page glue into
  // #range-slot after the charts' first paint; the glue shows and hides the slot. The fields are
  // fixed per workout, in a fixed grid, so the height never changes during a drag. The selection
  // listener writes text only (F-05 D-10): one O(1) query, about nine strings.
  import { onMount, untrack } from 'svelte';
  import type { WorkoutData } from '../../lib/datafile';
  import type { SelectionStore } from '../../lib/selection';
  import { createRangeStats, emptyValues, type RangeField, type RangeValues } from '../../lib/range';
  import {
    MISSING_TEXT,
    cadenceUnit,
    distanceUnit,
    elevationUnit,
    formatDistance,
    formatElapsed,
    formatFixed,
    formatSpeedOrPace,
    isPaceSport,
    speedOrPaceUnit,
    temperatureUnit,
    toElevation,
    toTemperature,
    type Units,
  } from '../../lib/format';

  interface Props {
    /** Fixed for the life of this instance. */
    data: WorkoutData;
    selection: SelectionStore;
    units?: Units;
    /** F-40 SR-7: the page's copy action for "Copy link to this selection"; no button without it
     *  (edit page). The panel knows no clipboard or URL rules. */
    oncopy?: (button: HTMLButtonElement) => void;
  }

  let { data, selection, units = 'metric', oncopy }: Props = $props();

  const sport = untrack(() => data.meta.sport);
  const stats = createRangeStats(untrack(() => data));
  const fields = stats.fields;
  const out = emptyValues();

  const LABELS: Readonly<Record<RangeField, string>> = {
    distance: 'Distance',
    elapsed: 'Time',
    moving: 'Moving',
    gain: 'Gain',
    speed: isPaceSport(sport) ? 'Avg pace' : 'Avg speed',
    heartRate: 'Avg HR',
    power: 'Avg power',
    cadence: 'Avg cadence',
    temperature: 'Avg temp',
  };

  function unitOf(f: RangeField, u: Units): string {
    switch (f) {
      case 'distance':
        return distanceUnit(u);
      case 'gain':
        return elevationUnit(u);
      case 'speed':
        return speedOrPaceUnit(sport, u);
      case 'heartRate':
        return 'bpm';
      case 'power':
        return 'W';
      case 'cadence':
        return cadenceUnit(sport);
      case 'temperature':
        return temperatureUnit(u);
      default:
        return '';
    }
  }

  function text(f: RangeField, v: RangeValues, u: Units): string {
    switch (f) {
      case 'distance':
        return formatDistance(v.distance, u);
      case 'elapsed':
      case 'moving':
        return formatElapsed(v[f]);
      case 'gain':
        return formatFixed(toElevation(v.gain, u), 0);
      case 'speed':
        return formatSpeedOrPace(sport, v.speed, u) || MISSING_TEXT;
      case 'temperature':
        return formatFixed(toTemperature(v.temperature, u), 0);
      default:
        return formatFixed(v[f], 0);
    }
  }

  let root: HTMLDivElement;
  let cells: HTMLElement[] = [];
  let current: Units = untrack(() => units);

  function write(a: number, b: number): void {
    if (a < 0 || cells.length === 0) return;
    stats.query(a, b, out);
    for (let k = 0; k < fields.length; k++) cells[k].textContent = text(fields[k], out, current);
  }

  onMount(() => {
    cells = [...root.querySelectorAll<HTMLElement>('.value')];
    return selection.subscribe(write);
  });

  // A units change rewrites the values (labels and units are markup).
  $effect(() => {
    current = units;
    untrack(() => write(selection.start(), selection.end()));
  });
</script>

<!-- The cell look (label line over value line, card) is shared with the cursor readout (page.css,
     "Value cards"); the auto-fill grid, the clipping and the action row are the panel's own. -->
<div class="range-panel" bind:this={root}>
  {#each fields as f (f)}
    <div class="cell">
      <div class="label">{LABELS[f]}</div>
      <div class="val"><span class="value">{MISSING_TEXT}</span>{#if unitOf(f, units)}&nbsp;<span class="unit">{unitOf(f, units)}</span>{/if}</div>
    </div>
  {/each}
  <div class="actions">
    <button type="button" class="clear" onclick={() => selection.clear()}>Clear</button>
    {#if oncopy}<button type="button" class="share" onclick={(e) => oncopy?.(e.currentTarget)}>Copy link to this selection</button>{/if}
  </div>
</div>

<style>
  .range-panel {
    grid-template-columns: repeat(auto-fill, minmax(64px, 1fr));
    align-items: center;
  }
  .cell,
  .label,
  .val {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  /* User 10 Nov 2026: the actions on their own full-width row, drawn as links (like the footer's
     Report button) to save space; the copy fallback field (.copy-url, a full line) wraps below. */
  .actions {
    grid-column: 1 / -1;
    display: flex;
    flex-wrap: wrap;
    column-gap: 16px;
  }
  .actions button {
    min-height: 24px;
    padding: 0;
    border: 0;
    background: none;
    color: var(--accent);
    font: inherit;
    font-size: 13px;
    text-decoration: underline;
    cursor: pointer;
  }
</style>
