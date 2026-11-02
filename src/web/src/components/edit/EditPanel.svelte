<script lang="ts">
  // The edit panel (F-19 design E-6 to E-8, OQ-81, OQ-82; F-20 C-2 to C-4, C-6, C-7, C-9 to C-11):
  // always open, one Save button, Publish, Share, Download original and Delete. Never sees the token.
  // An uncontrolled form: one attachment fills it from each stored settings object (so a save
  // shows what the server stored), another wires plain listeners, and Save reads the form. No
  // bind:, value=, checked= or on*= directives: their runtime would land in the activity entry,
  // which this chunk shares (F-19 bundle share).
  import type { Attachment } from 'svelte/attachments';
  import type { Settings, SettingsPatch } from '../../lib/owner';
  import { enhanceCopyLink } from '../../page/enhance';
  import DeleteControl from './DeleteControl.svelte';
  import { DESCRIPTION_MAX, TITLE_MAX, expiryText, incompleteNotice, lifetimeOptions, patchOf, type Form } from './fields';

  interface Props {
    settings: Settings;
    /** The current visibility; a restore can change it without a new settings object (C-2). */
    visibility: Settings['visibility'];
    /** The default title (UP-4), shown while the title is empty. */
    placeholder: string;
    /** '' or the running action's text ('Saving…'); every action is disabled meanwhile (C-2). */
    busy: string;
    /** 'Saved', 'No changes', 'Published' or the server's message; cleared by any edit. */
    status: string;
    now: number;
    /** location.origin, for the viewing link (C-4). */
    origin: string;
    onsave: (patch: SettingsPatch) => void;
    /** Publish with the form's unsaved changes (C-3, OQ-85). */
    onpublish: (patch: SettingsPatch) => void;
    /** The confirmed delete (C-7). */
    ondelete: () => void;
    ondownload: () => void;
    /** "Show links": the published page (OQ-86). */
    onlinks: () => void;
    /** The hiding checkbox changed, saved or not (E-9: the map marks follow it). */
    onhide: (on: boolean) => void;
    /** The owner changed a field after a save or an error. */
    onedit: () => void;
  }
  let { settings, visibility, placeholder, busy, status, now, origin, onsave, onpublish, ondelete, ondownload, onlinks, onhide, onedit }: Props = $props();

  let share = $state(false);
  const draft = $derived(visibility === 'draft');

  // svelte-ignore state_referenced_locally
  let lifetime = $state(settings.lifetime_days);
  // svelte-ignore state_referenced_locally
  const days = lifetimeOptions(settings.created_at, now);
  // svelte-ignore state_referenced_locally
  const notice = incompleteNotice(settings.data_end_s);

  type Field = HTMLInputElement & HTMLTextAreaElement & HTMLSelectElement;
  const field = (f: HTMLFormElement, name: string): Field => f.elements.namedItem(name) as Field;

  /** Re-runs for every new settings object (the attachment reads the prop). */
  const fill: Attachment<HTMLFormElement> = (f) => {
    const s = settings;
    field(f, 'title').value = s.title;
    field(f, 'description').value = s.description;
    field(f, 'hide_ends').checked = s.hide_ends;
    field(f, 'allow_download').checked = s.allow_download;
    field(f, 'lifetime').value = String(s.lifetime_days);
    lifetime = s.lifetime_days;
  };

  const read = (f: HTMLFormElement): Form => ({
    title: field(f, 'title').value,
    description: field(f, 'description').value,
    hide_ends: field(f, 'hide_ends').checked,
    allow_download: field(f, 'allow_download').checked,
    // '' when the stored lifetime is not offered (a browser clock ahead of the server's): keep it.
    lifetime_days: Number(field(f, 'lifetime').value) || settings.lifetime_days,
  });

  /** Runs once: it reads props only inside the listeners. */
  const wire: Attachment<HTMLFormElement> = (f) => {
    f.addEventListener('submit', (e) => {
      e.preventDefault();
      onsave(patchOf(settings, read(f)));
    });
    f.addEventListener('input', () => onedit());
    f.addEventListener('click', (e) => {
      const act = (e.target as Element).closest<HTMLElement>('[data-act]')?.dataset.act;
      if (act === 'publish') onpublish(patchOf(settings, read(f)));
      else if (act === 'share') share = !share;
      else if (act === 'download') ondownload();
      else if (act === 'links') onlinks();
    });
    f.addEventListener('change', (e) => {
      const t = e.target as Field;
      if (t.name === 'hide_ends') onhide(t.checked);
      if (t.name === 'lifetime') lifetime = Number(t.value);
    });
  };
</script>

<form class="edit-panel" aria-label="Settings" {@attach fill} {@attach wire}>
  {#if notice}<p class="edit-notice">{notice}</p>{/if}
  <!-- Locked while an action runs: Publish reads the form once, so a later change would be dropped (QA-20-r2-1). -->
  <fieldset disabled={busy !== ''}>
  <label class="edit-field">Title
    <input type="text" name="title" maxlength={TITLE_MAX} {placeholder} />
  </label>
  <label class="edit-field">Description
    <textarea name="description" maxlength={DESCRIPTION_MAX} rows="3"></textarea>
  </label>
  <label class="edit-check"><input type="checkbox" name="hide_ends" /> Hide start and end</label>
  <label class="edit-check"><input type="checkbox" name="allow_download" /> Viewers may download the file</label>
  <div class="edit-row">
    <span class="edit-field">Visibility <span class="edit-value">{draft ? 'Draft' : 'Unlisted'}</span></span>
    <label class="edit-field">Keep for
      <span class="edit-value"><select name="lifetime">
        {#each days as d}<option>{d}</option>{/each}
      </select> days</span>
    </label>
    <span class="edit-expiry">Expires {expiryText(settings.created_at, lifetime)}</span>
  </div>
  </fieldset>
  <div class="edit-row">
    <button type="submit" class="edit-save" disabled={busy !== ''}>Save</button>
    {#if draft}<button type="button" class="edit-save" data-act="publish" disabled={busy !== ''}>Publish</button>{/if}
    <span class="edit-status" role="status">{busy || status}</span>
  </div>
  {#if draft}<p class="edit-small">Unpublished drafts are deleted 48 hours after upload.</p>{/if}
  <div class="edit-row">
    {#if !draft}<button type="button" class="edit-button" data-act="share">Share</button>{/if}
    <button type="button" class="edit-button" data-act="download" disabled={busy !== ''}>Download original</button>
    <DeleteControl {busy} {ondelete} />
  </div>
  {#if share && !draft}
    <p class="edit-share" {@attach (p) => enhanceCopyLink(p, origin, navigator)}>
      <span class="edit-link">{origin + settings.view_url}</span>
      <a class="copy" href={settings.view_url} data-copy-link>Copy link</a>
      <button type="button" class="edit-button" data-act="links" disabled={busy !== ''}>Show links</button>
    </p>
  {/if}
  <p class="edit-small">Copies that viewers have already downloaded, and previews that messengers have already made, cannot be recalled.</p>
</form>
