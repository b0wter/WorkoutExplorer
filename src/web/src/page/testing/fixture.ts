// TEST-ONLY: a page in jsdom built the way the edit page builds it, from a committed sample view
// model (F-13 design D-3, D-9). Never imported by non-test code.

import { mount, unmount } from 'svelte';
import ActivityPage from '../../components/page/ActivityPage.svelte';
import type { PageViewModel } from '../../lib/page';
import { readSample } from '../../../test-support/samples.mjs';

export function sampleModel(name: string): PageViewModel {
  const text = readSample(name);
  if (text === null) throw new Error(`no sample ${name}`);
  return JSON.parse(text) as PageViewModel;
}

export interface Fixture {
  readonly model: PageViewModel;
  readonly target: HTMLElement;
  remove(): void;
}

/** Renders the sample into document.body and adds the data file preload link to document.head. */
export function renderSample(name: string, options: { report?: boolean; route?: PageViewModel['Route'] } = {}): Fixture {
  const model = sampleModel(name);
  if (options.route) (model as { Route: PageViewModel['Route'] }).Route = options.route;
  const link = document.createElement('link');
  link.id = 'wx-data';
  link.setAttribute('rel', 'preload');
  link.setAttribute('href', model.Page.DataURL);
  document.head.appendChild(link);
  const target = document.createElement('div');
  document.body.appendChild(target);
  const app = mount(ActivityPage, { target, props: { model, report: options.report ?? true } });
  return {
    model,
    target,
    remove() {
      unmount(app);
      target.remove();
      link.remove();
    },
  };
}
