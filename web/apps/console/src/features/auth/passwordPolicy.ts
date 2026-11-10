/**
 * Client-side mirror of the server password policy (login design spec §6,
 * internal/password/policy.go). The server stays authoritative; this module
 * only lets the change-password form reject violations inline before a
 * network round-trip. Weak-password-list and history-reuse checks remain
 * server-side and surface as field errors on new_password.
 */

export const POLICY_MIN_LENGTH_MULTI_CLASS = 12;
export const POLICY_MIN_LENGTH_SINGLE_CLASS = 16;

export type PasswordPolicyIssue = 'tooShort' | 'containsUsername';

/** Character classes per the server policy: lower / upper / digit / symbol. */
export function countPasswordClasses(password: string): number {
  let lower = false;
  let upper = false;
  let digit = false;
  let symbol = false;
  for (const ch of password) {
    if (/\p{Ll}/u.test(ch)) {
      lower = true;
    } else if (/\p{Lu}/u.test(ch)) {
      upper = true;
    } else if (/\p{Nd}/u.test(ch)) {
      digit = true;
    } else {
      symbol = true;
    }
  }
  return [lower, upper, digit, symbol].filter(Boolean).length;
}

/** Minimum rune length for the given password: 12 with ≥2 classes, else 16. */
export function passwordMinLength(password: string): number {
  return countPasswordClasses(password) >= 2
    ? POLICY_MIN_LENGTH_MULTI_CLASS
    : POLICY_MIN_LENGTH_SINGLE_CLASS;
}

/**
 * First client-checkable policy violation for `password`, or null when it
 * passes. username may be empty (containment check is case-insensitive,
 * matching the server).
 */
export function passwordPolicyIssue(
  password: string,
  username: string,
): PasswordPolicyIssue | null {
  if (Array.from(password).length < passwordMinLength(password)) {
    return 'tooShort';
  }
  if (username !== '' && password.toLowerCase().includes(username.toLowerCase())) {
    return 'containsUsername';
  }
  return null;
}
