// Text rules shared by the edit panel (F-19) and the report form (F-28 RF-4): one function, no copy.
// Its own module, so the report chunk does not pull in the edit panel's rules.

/** Unicode control characters (Cc) become spaces, so the rest of the text is never lost (UP-4);
 *  the description keeps its line feeds. */
export function cleanText(text: string, keepLineFeed: boolean): string {
  return text.replace(keepLineFeed ? /[\0-\t\v-\x1f\x7f-\x9f]/g : /[\0-\x1f\x7f-\x9f]/g, ' ');
}
