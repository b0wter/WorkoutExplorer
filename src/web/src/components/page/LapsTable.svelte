<script lang="ts">
  import { t } from './texts';
  // Laps table (AP-8), as in the template. Without laps but with an advancing distance the module
  // is hidden, headed "Splits", with no table, for the splits chunk (F-46 LS-3); without either there
  // is no module. The heading row has an empty slot for the chunk's switch or drop-down (LS-4). A multisport workout's rows sit
  // in one block per leg, heading and own table (F-64 LL-2).
  import type { PageLaps } from '../../lib/page';

  interface Props {
    laps: PageLaps | null;
  }
  let { laps }: Props = $props();
</script>

{#snippet table(columns: readonly { Key: string; Label: string }[], rows: readonly (readonly string[])[])}
  <div class="laps-scroll">
    <table>
      <thead><tr>{#each columns as column (column.Key)}<th data-k={column.Key}>{column.Label}</th>{/each}</tr></thead>
      <tbody>
        {#each rows as row, i (i)}
          <tr>{#each row as cell, j (j)}<td>{cell}</td>{/each}</tr>
        {/each}
      </tbody>
    </table>
  </div>
{/snippet}

{#if laps}
  <section class="laps" hidden={!!laps.None}>
    <div class="laps-head"><h2>{t(laps.None ? 'headSplits' : 'headLaps')}</h2><span class="laps-slot" id="laps-slot"></span></div>
    {#if !laps.None}
      {#if laps.Legs.length}
        <div class="laps-legs">
          {#each laps.Legs as leg, k (k)}
            <div class="laps-leg" data-sport={leg.Sport}>
              <h3>{leg.Name}</h3>
              {#if leg.Columns.length}{@render table(leg.Columns, leg.Rows)}{/if}
            </div>
          {/each}
        </div>
      {:else}
        {@render table(laps.Columns, laps.Rows)}
      {/if}
    {/if}
  </section>
{/if}
