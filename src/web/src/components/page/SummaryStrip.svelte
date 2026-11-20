<script lang="ts">
  import { t } from './texts';
  import { t as tp } from '../../lib/page/texts';
  // Summary strip (AP-2) with the privacy notice (SH-9), as in the template.
  import type { SummaryItem } from '../../lib/page';

  interface Props {
    items: readonly SummaryItem[];
    /** F-43 DI-7/DI-8: the device line, '' for none. Printed as text only (ADR-026). */
    device: string;
    /** F-58 SE-2: the trim string, the ⓘ button's data attribute. */
    trim: string;
  }
  let { items, device, trim }: Props = $props();
</script>

<section class="summary">
  <button type="button" class="summary-info" data-summary-info data-trim={trim} hidden title={t('summaryInfo')} aria-label={t('summaryInfo')}>ⓘ</button>
  {#each items as item (item.Key)}
    <div class="metric" data-k={item.Key}>
      <div class="k">{item.Label}</div>
      <div class="v">
        <span class="n">{item.Value}</span>{#if item.Unit}&nbsp;<span class="u">{item.Unit}</span>{/if}{#if item.Max}<span class="max">{tp('max', { v: item.Max })}</span>{/if}
      </div>
    </div>
  {/each}
</section>
{#if device}<p class="device">{device}</p>{/if}
<p class="notice">{t('privacyNotice')}</p>
