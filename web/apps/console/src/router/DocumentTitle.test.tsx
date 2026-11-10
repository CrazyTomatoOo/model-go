import { cleanup, render, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import i18n from '../i18n';
import DocumentTitle from './DocumentTitle';

describe('DocumentTitle', () => {
  beforeEach(async () => {
    // Deterministic start language; the suites share the i18n singleton.
    await i18n.changeLanguage('zh');
  });

  afterEach(async () => {
    cleanup();
    await i18n.changeLanguage('zh');
  });

  it('sets the route title from i18n copy', () => {
    render(
      <MemoryRouter initialEntries={['/']}>
        <DocumentTitle />
      </MemoryRouter>,
    );
    expect(document.title).toBe('控制台总览 · ModelGo 控制台');
  });

  it('refreshes the title on language change without navigation (#37)', async () => {
    render(
      <MemoryRouter initialEntries={['/']}>
        <DocumentTitle />
      </MemoryRouter>,
    );
    expect(document.title).toBe('控制台总览 · ModelGo 控制台');

    // The language toggle must re-render the title in place — the route
    // (pathname) does not change.
    await i18n.changeLanguage('en');
    await waitFor(() => {
      expect(document.title).toBe('Console Overview · ModelGo Console');
    });

    await i18n.changeLanguage('zh');
    await waitFor(() => {
      expect(document.title).toBe('控制台总览 · ModelGo 控制台');
    });
  });
});
