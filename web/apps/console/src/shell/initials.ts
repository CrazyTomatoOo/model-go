/**
 * Avatar initials derivation for the shell user menu (#36).
 *
 * The `/auth/me` identity is expected to carry a `display_name` (see
 * ConsoleUser.display_name); when the backend contract gains the field, the
 * avatar upgrades with no code change — until then the username is the
 * fallback, which is what the current backend actually returns.
 *
 * Rules: leading/trailing whitespace is ignored; two or more words take the
 * first letter of the first two words (code-point safe); a single word takes
 * its first character (covers CJK given names like 张三丰 → 张).
 */
export function getInitials(name: string | null | undefined): string {
  const trimmed = (name ?? '').trim();
  if (trimmed === '') {
    return '?';
  }
  const words = trimmed.split(/\s+/);
  const first = Array.from(words[0])[0] ?? '';
  if (words.length === 1) {
    return first.toUpperCase();
  }
  const second = Array.from(words[1])[0] ?? '';
  return (first + second).toUpperCase();
}
