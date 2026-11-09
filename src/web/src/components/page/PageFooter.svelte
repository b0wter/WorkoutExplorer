<script lang="ts">
  // Footer (INF-1, INF-2; F-38 IM-7: the Impressum link copied from edit.html). The edit page has no Report button (design D-9, decision 21; F-28 RF-1).
  import { impressum } from '../../lib/page/impressum';
  interface Props {
    report?: boolean;
  }
  let { report = true }: Props = $props();
</script>

<footer class="foot" {@attach impressum}><a href="/about">About</a>{#if report} <button type="button" data-report hidden>Report</button>{/if}</footer>
