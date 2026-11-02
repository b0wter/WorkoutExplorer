<script lang="ts">
  // Footer (INF-1, INF-2). The edit page has no Report link (design D-9, decision 21).
  interface Props {
    report?: boolean;
  }
  let { report = true }: Props = $props();
</script>

<footer class="foot"><a href="/about">About</a>{#if report} <a href="#report" data-report>Report</a>{/if}</footer>
