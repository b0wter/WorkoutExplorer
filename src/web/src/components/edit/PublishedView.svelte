<script lang="ts">
  // The published page (F-20 C-5, AC-6): the viewing link with Copy (C-4) and Go there, the edit link
  // with Copy, Go there (a button, lib/owner navigates) and the save warning, and Delete. No other workout data.
  // CRITICAL: the edit link is the only place the token is in the DOM: one text node in a
  // translate="no" element; never an attribute, an input or a link. Its copy fallback is text only.
  import type { Attachment } from 'svelte/attachments';
  import { enhanceCopyLink } from '../../page/enhance';
  import DeleteControl from './DeleteControl.svelte';

  interface Props {
    /** location.origin, for the viewing link. */
    origin: string;
    /** The settings' view_url (ADR-020 item 8). */
    viewUrl: string;
    /** lib/owner's edit link. */
    editLink: string;
    busy: string;
    /** The server's message after a failed delete. */
    status: string;
    onback: () => void;
    ondelete: () => void;
  }
  let { origin, viewUrl, editLink, busy, status, onback, ondelete }: Props = $props();
  let copied = $state('');

  /** Runs once: it reads props only inside the listener. */
  const wire: Attachment<HTMLElement> = (el) => {
    el.addEventListener('click', (e) => {
      const act = (e.target as Element).closest<HTMLElement>('[data-act]')?.dataset.act;
      if (act === 'back') onback();
      else if (act === 'copy-edit') {
        const write = navigator.clipboard ? navigator.clipboard.writeText(editLink) : Promise.reject(new Error('no clipboard'));
        write.then(
          () => (copied = 'Edit link copied.'),
          () => (copied = 'Copying did not work. Select the link above and copy it by hand.'),
        );
      }
    });
  };
</script>

<main class="generic published" {@attach wire}>
  <h1>Workout published</h1>
  <section class="link-box view-box">
    <h2>Viewing link</h2>
    <p class="edit-share" {@attach (p) => enhanceCopyLink(p, origin, navigator)}>
      <span class="edit-link">{origin + viewUrl}</span>
      <a class="copy" href={viewUrl} data-copy-link>Copy link</a>
      <a class="go" href={viewUrl}>Go there</a>
    </p>
  </section>
  <section class="link-box edit-warn">
    <h2>Edit link</h2>
    <p><strong>Save this link. It is the only way to change or delete this workout, and it cannot be recovered.</strong></p>
    <p class="edit-link" translate="no">{editLink}</p>
    <p><button type="button" class="edit-button" data-act="copy-edit">Copy edit link</button>
      <button type="button" class="edit-button" data-act="back" disabled={busy !== ''}>Go there</button> <span class="edit-copied" role="status">{copied}</span></p>
  </section>
  <p class="edit-delete-hint">Uploaded accidentally?</p>
  <p class="edit-row">
    <DeleteControl {busy} {ondelete} />
    <span class="edit-status" role="status">{busy || status}</span>
  </p>
</main>
<footer class="foot generic-foot"><a href="/about">About</a></footer>
