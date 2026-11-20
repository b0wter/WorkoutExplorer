// DOM glue of the upload page (F-11 design U-5 to U-10). Everything the browser is asked to do
// comes in through `Env`, so jsdom tests can replace it. No cookies; browser storage only for the
// language drop-down's choice (F-54 LG-1, through `Env.remember`).
import { canPick, MAX_UPLOAD_BYTES, outcomeOf, parseStatus, percent, t, tooLarge, type PageState } from '../lib/upload';
import { onLanguage } from '../lib/i18n';
import { canInflate, extractFit, isZip } from '../lib/zip';

export interface Env {
  fetch(input: string, init?: RequestInit): Promise<Response>;
  xhr(): XMLHttpRequest;
  navigate(url: string): void;
  origin: string;
  /** Longest wait for `/api/status` before the page fails open (U-5). */
  statusWaitMs: number;
}

function el<T extends HTMLElement>(doc: Document, id: string): T {
  const e = doc.getElementById(id);
  if (!e) throw new Error(`missing #${id}`);
  return e as T;
}

/** Starts the page; the returned function removes the document and window listeners (tests). */
export function start(doc: Document, win: Window, env: Env): () => void {
  const page = el(doc, 'page');
  const notice = el(doc, 'notice');
  const drop = el(doc, 'drop');
  const input = el<HTMLInputElement>(doc, 'file');
  const progress = el(doc, 'progress');
  const message = el(doc, 'message');

  let state: PageState = 'checking';
  let generation = 0;

  type Lines = { notice?: () => string; progress?: () => string; message?: () => string };
  let lines: Lines = {};
  const draw = (): void => {
    notice.textContent = lines.notice?.() ?? '';
    progress.textContent = lines.progress?.() ?? '';
    message.textContent = lines.message?.() ?? '';
  };
  // Lines are kept as functions so a language change redraws what is on screen (F-54 AC "applies at once").
  function show(next: PageState, opts: Lines = {}): void {
    state = next;
    page.dataset.state = next;
    input.disabled = !canPick(next);
    lines = opts;
    draw();
  }
  const offLang = onLanguage(draw);

  async function checkStatus(): Promise<void> {
    const gen = ++generation;
    show('checking');
    let open = true; // fails open: a broken status never blocks uploads (U-5)
    const ctl = new AbortController();
    const timer = win.setTimeout(() => ctl.abort(), env.statusWaitMs);
    try {
      const res = await env.fetch('/api/status', { cache: 'no-store', signal: ctl.signal });
      if (res.ok) open = parseStatus(await res.text()) !== false;
    } catch {
      // network error, timeout: stay open
    } finally {
      win.clearTimeout(timer);
    }
    if (gen !== generation) return;
    if (open) show('ready');
    else show('paused', { notice: () => t('paused') });
  }

  function pick(file: File | undefined): void {
    if (!file || !canPick(state)) return; // a second pick or drop while busy starts nothing
    if (tooLarge(file.size)) {
      show('ready', { message: () => t('tooLarge') });
      return;
    }
    void open(file);
  }

  // F-35 Z-3, Z-9 to Z-11: busy from the first byte read; an archive is replaced by its FIT file.
  async function open(file: File): Promise<void> {
    const gen = generation;
    show('opening');
    let bytes: Uint8Array<ArrayBuffer>;
    try {
      if (!isZip(new Uint8Array(await file.slice(0, 4).arrayBuffer()))) {
        if (gen === generation) send(file); // a FIT file is sent as the picked File, unchanged
        return;
      }
      if (gen !== generation) return;
      if (!canInflate()) return show('ready', { message: () => t('noZip') });
      lines = { ...lines, progress: () => t('opening') };
      draw();
      bytes = new Uint8Array(await file.arrayBuffer());
    } catch {
      if (gen === generation) show('ready', { message: () => t('uploadUnreadable') });
      return;
    }
    if (gen !== generation) return;
    const r = await extractFit(bytes, MAX_UPLOAD_BYTES);
    if (gen !== generation) return;
    if (r.ok) send(new File(r.chunks as BlobPart[], 'workout.fit'));
    else show('ready', { message: () => t(r.reason) });
  }

  function send(file: File): void {
    const gen = generation;
    const xhr = env.xhr();
    show('uploading', { progress: () => t('uploadingPercent', { n: 0 }) });
    const fail = (text: () => string): void => {
      if (gen === generation) show('ready', { message: text });
    };
    xhr.open('POST', '/api/workouts');
    xhr.upload.onprogress = (e) => {
      if (gen === generation && state === 'uploading' && e.lengthComputable) {
        const n = percent(e.loaded, e.total);
        lines = { progress: () => t('uploadingPercent', { n }) };
        draw();
      }
    };
    xhr.upload.onload = () => {
      if (gen === generation) show('processing', { progress: () => t('reading') });
    };
    xhr.onload = () => {
      if (gen !== generation) return;
      const out = () => outcomeOf(xhr.status, xhr.responseText, env.origin);
      const msg = () => {
        const o = out();
        return o.kind === 'navigate' ? '' : o.message;
      };
      const first = out();
      if (first.kind === 'navigate') env.navigate(first.url);
      else if (first.kind === 'paused') show('paused', { notice: msg });
      else fail(msg);
    };
    xhr.onerror = xhr.onabort = () => fail(() => t('uploadUnreadable'));
    const form = new FormData();
    form.append('file', file);
    xhr.send(form);
  }

  const onChange = () => {
    const file = input.files?.[0];
    input.value = ''; // the same file can be picked again after a refusal (U-6)
    pick(file);
  };
  input.addEventListener('change', onChange);

  // A file dropped anywhere never makes the browser open it (U-6); only the first file counts.
  const onDragOver = (e: Event) => {
    e.preventDefault();
    drop.classList.add('over');
  };
  const onDragLeave = () => drop.classList.remove('over');
  const onDrop = (e: DragEvent) => {
    e.preventDefault();
    drop.classList.remove('over');
    pick(e.dataTransfer?.files?.[0]);
  };
  doc.addEventListener('dragover', onDragOver);
  doc.addEventListener('dragleave', onDragLeave);
  doc.addEventListener('drop', onDrop);

  // Back-forward cache: start again, status included (U-5).
  const onPageShow = (e: Event) => {
    if ((e as PageTransitionEvent).persisted) void checkStatus();
  };
  win.addEventListener('pageshow', onPageShow);

  void checkStatus();
  return () => {
    input.removeEventListener('change', onChange);
    doc.removeEventListener('dragover', onDragOver);
    doc.removeEventListener('dragleave', onDragLeave);
    doc.removeEventListener('drop', onDrop);
    win.removeEventListener('pageshow', onPageShow);
    offLang();
  };
}
