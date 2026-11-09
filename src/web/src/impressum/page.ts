// The Impressum page (F-38 IM-2, IM-3; INF-7). Nothing is fetched on load: only a click on the button
// fetches the text, once per click. The body is the server's escaped fragment (h2, p, ul; IM-4).

const FAILED = 'The Impressum could not be loaded. Reload the page.'; // OQ-124

type Fetch = (input: string, init?: RequestInit) => Promise<Response>;

export const REQUEST: RequestInit = {
  headers: { 'Wx-Impressum': '1' }, // IM-5
  mode: 'same-origin',
  credentials: 'same-origin',
  referrerPolicy: 'no-referrer',
  cache: 'no-store',
  redirect: 'error',
};

export function start(doc: Document, fetchFn: Fetch): void {
  const button = doc.getElementById('show') as HTMLButtonElement;
  const text = doc.getElementById('text')!;
  const message = doc.getElementById('message')!;
  button.hidden = false;
  button.addEventListener('click', async () => {
    if (button.disabled) return;
    button.disabled = true;
    message.textContent = '';
    try {
      const r = await fetchFn('/api/impressum', REQUEST);
      if (!r.ok) throw new Error(String(r.status));
      text.innerHTML = await r.text();
      button.remove();
    } catch {
      message.textContent = FAILED;
      button.disabled = false;
    }
  });
}
