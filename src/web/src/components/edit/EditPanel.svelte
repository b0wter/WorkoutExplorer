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
  import { formatElapsed } from '../../lib/format';
  import { DESCRIPTION_MAX, TITLE_MAX, TRIM_MIN_S, expiryText, incompleteNotice, lifetimeOptions, patchOf, trimAt, trimClamp, trimIndex, type Form } from './fields';

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
    /** F-47 TR-9: draft.bin's time offsets and start_time for the trim slider; null without a
     *  decoded draft (no slider, the stored trim goes back unchanged). */
    time: Float64Array | null;
    startTime: string;
    /** The trim handles as sample indices [a, b], on every move and fill, saved or not. */
    ontrim: (a: number, b: number) => void;
  }
  let { settings, visibility, placeholder, busy, status, now, origin, onsave, onpublish, ondelete, ondownload, onlinks, onhide, onedit, time, startTime, ontrim }: Props = $props();

  let share = $state(false);
  const draft = $derived(visibility === 'draft');

  // svelte-ignore state_referenced_locally
  let lifetime = $state(settings.lifetime_days);
  // svelte-ignore state_referenced_locally
  const days = lifetimeOptions(settings.created_at, now);
  // svelte-ignore state_referenced_locally
  const notice = incompleteNotice(settings.data_end_s);

  // F-47 TR-9: the slider, only when a trim can leave 60 s (TR-4).
  // svelte-ignore state_referenced_locally
  const trim = time && time[time.length - 1] - time[0] >= TRIM_MIN_S ? time : null;
  // svelte-ignore state_referenced_locally
  const startS = Date.parse(startTime) / 1000;
  /** The handles, for the elapsed-time text. */
  let cut = $state<[number, number]>([0, 0]);
  const elapsed = (t: Float64Array, i: number): string => formatElapsed(t[i] - t[0]);

  type Field = HTMLInputElement & HTMLTextAreaElement & HTMLSelectElement;
  const field = (f: HTMLFormElement, name: string): Field => f.elements.namedItem(name) as Field;

  /** Sets both handles and tells the page (the shading follows them). */
  const setCut = (f: HTMLFormElement, a: number, b: number): void => {
    field(f, 'trim_a').value = String(a);
    field(f, 'trim_b').value = String(b);
    cut = [a, b];
    ontrim(a, b);
  };

  /** Re-runs for every new settings object (the attachment reads the prop). */
  const fill: Attachment<HTMLFormElement> = (f) => {
    const s = settings;
    field(f, 'title').value = s.title;
    field(f, 'description').value = s.description;
    field(f, 'hide_ends').checked = s.hide_ends;
    field(f, 'lifetime').value = String(s.lifetime_days);
    lifetime = s.lifetime_days;
    if (trim) setCut(f, trimIndex(trim, startS, s.trim_start_at, false), trimIndex(trim, startS, s.trim_end_at, true));
  };

  const read = (f: HTMLFormElement): Form => ({
    title: field(f, 'title').value,
    description: field(f, 'description').value,
    hide_ends: field(f, 'hide_ends').checked,
    // Hidden until F-24 (phase 3): the stored value goes back unchanged, so a save never flips it.
    allow_download: settings.allow_download,
    // '' when the stored lifetime is not offered (a browser clock ahead of the server's): keep it.
    lifetime_days: Number(field(f, 'lifetime').value) || settings.lifetime_days,
    trim_start_at: trim ? trimAt(trim, startS, Number(field(f, 'trim_a').value), false) : settings.trim_start_at,
    trim_end_at: trim ? trimAt(trim, startS, Number(field(f, 'trim_b').value), true) : settings.trim_end_at,
  });

  /** Runs once: it reads props only inside the listeners. */
  const wire: Attachment<HTMLFormElement> = (f) => {
    f.addEventListener('submit', (e) => {
      e.preventDefault();
      onsave(patchOf(settings, read(f)));
    });
    f.addEventListener('input', (e) => {
      onedit();
      const t = e.target as Field;
      if (trim && t.name.startsWith('trim_')) setCut(f, ...trimClamp(trim, Number(field(f, 'trim_a').value), Number(field(f, 'trim_b').value), t.name === 'trim_b'));
    });
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
  {#if trim}
    <div class="edit-field">Trim start and end
      <div class="edit-trim">
        <input type="range" name="trim_a" aria-label="Trim start" min="0" max={trim.length - 1} step="1" />
        <input type="range" name="trim_b" aria-label="Trim end" min="0" max={trim.length - 1} step="1" />
      </div>
      <span class="edit-value">{elapsed(trim, cut[0])} – {elapsed(trim, cut[1])}</span>
    </div>
  {/if}
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
