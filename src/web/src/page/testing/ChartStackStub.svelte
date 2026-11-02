<script lang="ts">
  // TEST-ONLY stand-in for ChartStack.svelte (Prototype.dom.test.ts, ADR-012 rev. 2): records
  // its props and its creation; renders nothing.
  import { stubLog, type ChartStackStubProps } from './stub-log';

  const props: ChartStackStubProps = $props();
  // Inside a function: the live props object is recorded, not a snapshot of one value.
  function record(): void {
    stubLog.calls.push('ChartStack');
    stubLog.chartStack.push(props);
  }
  record();
</script>
