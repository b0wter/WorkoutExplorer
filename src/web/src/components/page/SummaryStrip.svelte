<script lang="ts">
  // Summary strip (AP-2) with the privacy notice (SH-9), as in the template.
  import type { SummaryItem } from '../../lib/page';

  interface Props {
    items: readonly SummaryItem[];
  }
  let { items }: Props = $props();
</script>

<section class="summary">
  {#each items as item (item.Key)}
    <div class="metric" data-k={item.Key}>
      <div class="k">{item.Label}</div>
      <div class="v">
        <span class="n">{item.Value}</span>{#if item.Unit}&nbsp;<span class="u">{item.Unit}</span>{/if}{#if item.Max}<span class="max">max {item.Max}</span>{/if}
      </div>
    </div>
  {/each}
</section>
<p class="notice">Parts of the route, such as the start and end, may be hidden for privacy. Totals include them.</p>
