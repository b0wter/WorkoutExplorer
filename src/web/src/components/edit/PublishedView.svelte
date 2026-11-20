<script lang="ts">
  // The published page (F-20 C-5, AC-6): the viewing link with Copy (C-4) and Go there, the edit link
  // with Copy, Go there (a button, lib/owner navigates) and the save warning, F-53's list note (AC 51), and Delete. No other workout data.
  // CRITICAL: the edit link is the only place the token is in the DOM: one text node in a
  // translate="no" element; never an attribute, an input or a link. Its copy fallback is text only.
  import type { Attachment } from 'svelte/attachments';
  import { impressum } from '../../lib/page/impressum';
  import { enhanceCopyLink } from '../../page/enhance';
  import DeleteControl from './DeleteControl.svelte';
  import { t } from '../../edit/texts';
  import { t as tc } from '../page/texts';

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
          () => (copied = t('editLinkCopied')),
          () => (copied = t('copyFailed')),
        );
      }
    });
  };
</script>

<main class="generic published" {@attach wire}>
  <h1>{t('published')}</h1>
  <section class="link-box view-box">
    <h2>{t('viewingLink')}</h2>
    <p class="edit-share" {@attach (p) => enhanceCopyLink(p, origin, navigator)}>
      <span class="edit-link">{origin + viewUrl}</span>
      <a class="copy" href={viewUrl} data-copy-link>{tc('copyLink')}</a>
      <a class="go" href={viewUrl}>{t('goThere')}</a>
    </p>
  </section>
  <section class="link-box edit-warn">
    <h2>{t('editLink')}</h2>
    <p><strong>{t('saveLinkWarning')}</strong></p>
    <p class="edit-link" translate="no">{editLink}</p>
    <p><button type="button" class="edit-button" data-act="copy-edit">{t('copyEditLink')}</button>
      <button type="button" class="edit-button" data-act="back" disabled={busy !== ''}>{t('goThere')}</button> <span class="edit-copied" role="status">{copied}</span></p>
  </section>
  <p>{t('linksSaved')}</p>
  <p class="edit-delete-hint">{t('accidental')}</p>
  <p class="edit-row">
    <DeleteControl {busy} {ondelete} />
    <span class="edit-status" role="status">{busy || status}</span>
  </p>
</main>
<footer class="foot generic-foot" {@attach impressum}><a href="/about">{tc('about')}</a></footer>
