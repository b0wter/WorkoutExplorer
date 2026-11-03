<script lang="ts">
  // Footer (INF-1, INF-2). The edit page has no Report button (design D-9, decision 21; F-28 RF-1).
  interface Props {
    report?: boolean;
  }
  let { report = true }: Props = $props();
</script>

<footer class="foot"><a href="/about">About</a>{#if report} <button type="button" data-report hidden>Report</button>{/if}</footer>
