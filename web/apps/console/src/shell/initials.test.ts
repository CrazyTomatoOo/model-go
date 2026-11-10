import { describe, expect, it } from 'vitest';
import { getInitials } from './initials';

describe('getInitials', () => {
  it('takes the first letter of the first two words', () => {
    expect(getInitials('Zhang San')).toBe('ZS');
    expect(getInitials('ada lovelace')).toBe('AL');
  });

  it('collapses repeated whitespace', () => {
    expect(getInitials('  Li   Ming  ')).toBe('LM');
  });

  it('takes the first character of a single word', () => {
    expect(getInitials('sysadmin')).toBe('S');
  });

  it('is code-point safe for CJK names', () => {
    expect(getInitials('张三丰')).toBe('张');
  });

  it('falls back for missing names', () => {
    expect(getInitials('')).toBe('?');
    expect(getInitials('   ')).toBe('?');
    expect(getInitials(null)).toBe('?');
    expect(getInitials(undefined)).toBe('?');
  });
});
