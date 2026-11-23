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

<!-- F-58 OQ-251 (user, 23 Nov 2026): a bar like How to use this page (page.css .howto), under it. -->
<div class="howto-row"><button type="button" class="howto" data-howto><svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="12" r="9"/><path d="M12 11v6M12 7.5h.01"/></svg><span>{t('howto')}</span></button></div>
<div class="howto-row"><button type="button" class="howto" data-summary-info data-trim={trim}><svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="12" r="9"/><path d="M12 11v6M12 7.5h.01"/></svg><span>{t('summaryInfo')}</span></button></div>
<section class="summary">
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
