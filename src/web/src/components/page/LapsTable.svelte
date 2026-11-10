<script lang="ts">
  // Laps table (AP-8), as in the template. Without laps but with an advancing distance the module
  // holds "No laps recorded." (F-18 L-8); without either there is no module. The heading row has
  // an empty slot for the bundle's laps / splits switch (F-18 L-6). A multisport workout's rows sit
  // under their legs' separator rows, one tbody per leg (F-42 MS-3, MS-6).
  import type { PageLaps } from '../../lib/page';

  interface Props {
    laps: PageLaps | null;
  }
  let { laps }: Props = $props();
</script>

{#if laps}
  <section class="laps">
    <div class="laps-head"><h2>Laps</h2><span class="laps-slot" id="laps-slot"></span></div>
    {#if laps.None}
      <p class="laps-none">No laps recorded.</p>
    {:else}
      <div class="laps-scroll">
        <table>
          <thead><tr>{#each laps.Columns as column (column.Key)}<th data-k={column.Key}>{column.Label}</th>{/each}</tr></thead>
          {#each laps.Sections.length ? laps.Sections : [{ Name: '', Span: '', Rows: laps.Rows }] as sec, k (k)}
            <tbody>
              {#if sec.Name}<tr class="laps-section"><th colspan={+sec.Span} scope="rowgroup">{sec.Name}</th></tr>{/if}
              {#each sec.Rows as row, i (i)}
                <tr>{#each row as cell, j (j)}<td>{cell}</td>{/each}</tr>
              {/each}
            </tbody>
          {/each}
        </table>
      </div>
    {/if}
  </section>
{/if}
