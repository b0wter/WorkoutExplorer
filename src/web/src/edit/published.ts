// The published page, lazy: only an edit link of a published workout needs it (ADR-011).
import { mount } from 'svelte';
import PublishedView from '../components/edit/PublishedView.svelte';
import { serverText } from '../lib/errors/texts';
import type { OwnerClient, Settings } from '../lib/owner';

/** The published page (C-5): the settings only; a draft goes to the edit page with replace. */
export async function startPublished(
  host: HTMLElement,
  owner: OwnerClient,
  origin: string,
  mineList: { update(s: Settings): void },
  gone: () => void,
  actions: () => { busy: string; status: string; remove(): Promise<void> },
  showMessage: (message: string) => void,
): Promise<void> {
  const doc = host.ownerDocument;
  const got = await owner.settings();
  if (!got.ok) return got.notFound ? gone() : showMessage(serverText({ error: got.code, message: got.message }));
  if (got.value.visibility === 'draft') return owner.toEdit(true);
  mineList.update(got.value);
  const ui = actions();
  doc.title = 'Workout Explorer';
  host.replaceChildren();
  mount(PublishedView, {
    target: host,
    props: {
      origin,
      viewUrl: got.value.view_url,
      editLink: owner.editLink!,
      get busy() {
        return ui.busy;
      },
      get status() {
        return ui.status;
      },
      onback: () => owner.toEdit(false),
      ondelete: () => void ui.remove(),
    },
  });
}
