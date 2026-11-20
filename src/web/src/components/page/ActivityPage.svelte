<script lang="ts">
  import { t } from './texts';
  import { t as tm } from '../map/texts';
  // Root of the static page modules for the edit page (F-13 design D-9; F-19 mounts it lazily).
  // Renders the template's markup from the view model; the map and chart slots stay empty, and
  // the same glue (page/start.ts) mounts the islands into them and attaches the enhancers.
  // The viewing entry never imports this folder.
  import type { PageViewModel } from '../../lib/page';
  import LapsTable from './LapsTable.svelte';
  import PageFooter from './PageFooter.svelte';
  import PageHeader from './PageHeader.svelte';
  import SummaryStrip from './SummaryStrip.svelte';

  interface Props {
    model: PageViewModel;
    /** false on the edit page. */
    report?: boolean;
    /** true on the edit page: an empty slot for the panel as the first item of the content
     *  column, above the header (F-19 E-10, OQ-81). */
    edit?: boolean;
    /** false while the edited workout is a draft: no Copy link (PageHeader). */
    copy?: boolean;
  }
  let { model, report = false, edit = false, copy = true }: Props = $props();
</script>

<div class="page" class:has-map={model.Route.Map !== ""}>
  <div class="layout">
  {#if edit}<div id="edit-slot" class="edit-slot"></div>{/if}
  <PageHeader page={model.Page} header={model.Header} {copy} />
  <SummaryStrip items={model.Summary} device={model.Device} trim={model.Trim} />
  {#if model.Route.Map}<div id="map-slot" class="map-slot"></div>{/if}
  {#if model.Pins.URL}<div class="pins-slot" data-url={model.Pins.URL} data-count={model.Pins.Count} hidden></div>{/if}
  {#if edit}<div class="pins-slot"></div>{/if}
  {#if model.Route.NoLocation}<p class="hint">{tm('noLocation')}</p>{/if}
  {#if model.Route.Hidden}<p class="hint">{t('routeHidden')}</p>{/if}
  {#if model.ChartsClass}<div id="charts-slot" class={'charts ' + model.ChartsClass}></div>{/if}
  <div id="range-slot" class="range-slot"></div>
  <LapsTable laps={model.Laps} />
  <div id="zones-slot" class="zones-slot" hidden></div>
  <div id="efforts-slot" class="efforts-slot" hidden></div>
  <PageFooter {report} />
  </div>
</div>
