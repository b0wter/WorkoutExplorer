<script lang="ts">
  // Delete with its in-page confirmation (F-20 C-7), on the edit page and the published page. Never
  // window.confirm. The confirmation is announced (role="alert") and takes the focus on Cancel, the
  // safe choice (QA-20-r1-3). No on*= directives: see EditPanel.
  import type { Attachment } from 'svelte/attachments';

  interface Props {
    /** '' or the running action's text; Delete and Confirm are disabled meanwhile (C-2). */
    busy: string;
    /** The confirmed delete. */
    ondelete: () => void;
  }
  let { busy, ondelete }: Props = $props();
  let confirming = $state(false);

  /** Runs once: it reads props only inside the listener. */
  const wire: Attachment<HTMLElement> = (el) => {
    el.addEventListener('click', (e) => {
      const act = (e.target as Element).closest<HTMLElement>('[data-act]')?.dataset.act;
      if (act === 'delete') confirming = true;
      else if (act === 'cancel') confirming = false;
      else if (act === 'confirm') ondelete();
    });
  };
</script>

<span class="edit-delete" {@attach wire}>
  <button type="button" class="edit-button" data-act="delete" disabled={busy !== ''}>Delete</button>
  {#if confirming}
    <span class="edit-confirm" role="alert">
      <span class="edit-confirm-text">Delete this workout permanently? Both the viewing link and the edit link stop working. This cannot be undone.</span>
      <span><button type="button" class="edit-button edit-danger" data-act="confirm" disabled={busy !== ''}>Delete permanently</button> <button type="button" class="edit-button" data-act="cancel" {@attach (b) => b.focus()}>Cancel</button></span>
    </span>
  {/if}
</span>
