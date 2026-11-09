/** F-38 IM-7: copies edit.html's Impressum link (template #wx-impressum, filled by the server or
 *  empty) after a browser-drawn footer's About link. No template, or an empty one: nothing. */
export function impressum(footer: HTMLElement): void {
  const t = footer.ownerDocument.getElementById('wx-impressum') as HTMLTemplateElement | null;
  if (t) footer.querySelector('a[href="/about"]')!.after(' ', t.content.cloneNode(true));
}
