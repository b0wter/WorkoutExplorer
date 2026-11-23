<script lang="ts">
  import { t } from './texts';
  // Laps table (AP-8), as in the template. Without laps but with an advancing distance the module
  // is hidden, headed "Splits", with no table, for the splits chunk (F-46 LS-3); without either there
  // is no module. The heading row has an empty slot for the chunk's switch or drop-down (LS-4). A multisport workout's rows sit
  // in one block per leg, heading and own table (F-64 LL-2).
  import type { PageLaps } from '../../lib/page';

  /** F-65 AL-8, edit page only: per lap in DOM order, its added boundary (a sample index, the ×'s
   *  data-at; the edit glue handles its click), the ×'s accessible name and the dimmed reason
   *  ('' = kept); null = a row without ×. */
  type LapRemoval = { readonly at: number; readonly name: string; readonly dim: string } | null;

  interface Props {
    laps: PageLaps | null;
    remove?: { readonly rows: readonly LapRemoval[] };
    /** F-66 RA-9, edit page only: the line under the table, "Remove auto-laps (N)" or "Restore auto-laps"; its click is the edit glue's. */
    auto?: { readonly on: boolean; readonly label: string };
  }
  let { laps, remove, auto }: Props = $props();
  const x = $derived(!!remove?.rows.some((r) => r));
  /** The first lap of each leg block (L-2's DOM order). */
  const offsets = $derived(laps?.Legs.reduce<number[]>((o, l, k) => [...o, (o[k - 1] ?? 0) + (laps!.Legs[k - 1]?.Rows.length ?? 0)], []) ?? []);
</script>

{#snippet table(columns: readonly { Key: string; Label: string }[], rows: readonly (readonly string[])[], offset: number)}
  <div class="laps-scroll">
    <table>
      <thead><tr>{#each columns as column (column.Key)}<th data-k={column.Key}>{column.Label}</th>{/each}{#if x}<th class="lap-x"></th>{/if}</tr></thead>
      <tbody>
        {#each rows as row, i (i)}
          {@const r = x ? remove!.rows[offset + i] : null}
          <tr class:dim={!!r?.dim}>{#each row as cell, j (j)}<td>{cell}</td>{/each}{#if x}<td class="lap-x">{#if r}<button type="button" class="lap-x-btn" data-at={r.at} aria-label={r.name} title={r.name}>×</button>{#if r.dim}<span class="edit-small">{r.dim}</span>{/if}{/if}</td>{/if}</tr>
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
              {#if leg.Columns.length}{@render table(leg.Columns, leg.Rows, offsets[k])}{/if}
            </div>
          {/each}
        </div>
      {:else}
        {@render table(laps.Columns, laps.Rows, 0)}
      {/if}
    {/if}
    {#if auto}<p class="laps-auto"><button type="button" class="laps-auto-btn">{auto.label}</button></p>{/if}
  </section>
{/if}
