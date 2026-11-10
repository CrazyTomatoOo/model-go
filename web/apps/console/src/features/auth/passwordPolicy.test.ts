import { describe, expect, it } from 'vitest';
import {
  countPasswordClasses,
  passwordMinLength,
  passwordPolicyIssue,
} from './passwordPolicy';

// Parity with internal/password/policy_test.go: the same cases must pass or
// fail identically on both sides of the API.
describe('passwordPolicyIssue', () => {
  it('accepts a two-class password at 12 characters', () => {
    expect(passwordPolicyIssue('Abcdef12345!', 'alice')).toBeNull();
  });

  it('accepts a four-class password', () => {
    expect(passwordPolicyIssue('Correct-Horse42!', 'alice')).toBeNull();
  });

  it('accepts a passphrase with symbols', () => {
    expect(passwordPolicyIssue('correct-horse-battery-staple-42!', 'alice')).toBeNull();
  });

  it('accepts a single-class password at 16 characters', () => {
    expect(passwordPolicyIssue('a'.repeat(16), 'alice')).toBeNull();
  });

  it('rejects a multi-class password below 12 characters', () => {
    expect(passwordPolicyIssue('Abcdef1234!', 'alice')).toBe('tooShort');
  });

  it('rejects a single-class password below 16 characters', () => {
    expect(passwordPolicyIssue('a'.repeat(15), 'alice')).toBe('tooShort');
  });

  it('rejects an empty password (single-class minimum applies)', () => {
    expect(passwordPolicyIssue('', 'alice')).toBe('tooShort');
    expect(passwordMinLength('')).toBe(16);
  });

  it('measures length in runes, matching the server', () => {
    // CJK ideographs are category Lo: symbol-class on the server, so a
    // CJK-only password needs the single-class minimum of 16 runes.
    expect(countPasswordClasses('密')).toBe(1);
    expect(passwordPolicyIssue('密'.repeat(15), '')).toBe('tooShort');
    expect(passwordPolicyIssue('密'.repeat(16), '')).toBeNull();
  });

  it('rejects a password containing the username', () => {
    expect(passwordPolicyIssue('aliceSecret12!a', 'alice')).toBe('containsUsername');
  });

  it('rejects a password containing the username case-insensitively', () => {
    expect(passwordPolicyIssue('ALICESecret12!a', 'alice')).toBe('containsUsername');
    expect(passwordPolicyIssue('aliceSecret12!a', 'ALICE')).toBe('containsUsername');
  });

  it('accepts an unrelated password for a username it does not contain', () => {
    expect(passwordPolicyIssue('Za1!zxcvbnm123', 'alice')).toBeNull();
  });

  it('skips the username check when no username is known', () => {
    expect(passwordPolicyIssue('aliceSecret12!a', '')).toBeNull();
  });
});

describe('countPasswordClasses', () => {
  it('counts lower/upper/digit/symbol', () => {
    expect(countPasswordClasses('abc')).toBe(1);
    expect(countPasswordClasses('abcABC')).toBe(2);
    expect(countPasswordClasses('abcABC123')).toBe(3);
    expect(countPasswordClasses('abcABC123!')).toBe(4);
  });
});
