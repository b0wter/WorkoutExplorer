<script lang="ts">
  // TEST-ONLY stand-in for RouteMap.svelte (Prototype.dom.test.ts, ADR-012 rev. 2): records
  // its props and its creation; renders nothing.
  import { stubLog, type RouteMapStubProps } from './stub-log';

  const props: RouteMapStubProps = $props();
  // Inside a function: the live props object is recorded, not a snapshot of one value.
  function record(): void {
    stubLog.calls.push('RouteMap');
    stubLog.routeMap.push(props);
  }
  record();
</script>
