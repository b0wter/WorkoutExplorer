<script lang="ts">
  // Header (AP-1) as Svelte, for the edit page (F-13 design D-9). The reference markup is the
  // template, src/web/index.html: keep them equal. Plain text only (AC 5).
  import type { HeaderGroup, PageGroup } from '../../lib/page';

  interface Props {
    page: PageGroup;
    header: HeaderGroup;
    /** false hides Copy link: a draft has no viewing page yet (F-19 user decision). */
    copy?: boolean;
  }
  let { page, header, copy = true }: Props = $props();
</script>

<header class="head">
  <svg class="sport" aria-hidden="true"><use href={'#sport-' + header.SportIcon} /></svg>
  <div class="head-text">
    <h1>{header.Title}</h1>
    <p class="when">{header.SportName} · {header.Start}</p>
    {#if header.Description}<p class="desc">{header.Description}</p>{/if}
    <p class="expires"><time data-expires datetime={header.ExpiresAt}>{header.ExpiresText}</time></p>
  </div>
  <div class="ctl"><a class="copy" href={page.ViewPath} hidden={!copy} data-copy-link>Copy link</a><span class="units-slot" id="units-slot"></span></div>
</header>
