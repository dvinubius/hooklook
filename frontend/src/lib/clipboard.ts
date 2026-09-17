/** Clipboard access, which is allowed to be unavailable.
 *
 * The Clipboard API needs a secure context and a user gesture, and either can
 * be missing. Every URL this application offers to copy is also on screen as
 * selectable text, so a refusal is reported and nothing else happens. */

export async function copyText(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text)
    return true
  } catch {
    return false
  }
}
