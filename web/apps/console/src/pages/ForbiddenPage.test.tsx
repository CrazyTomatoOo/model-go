import { act, cleanup, render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import '../i18n';
import ForbiddenPage from './ForbiddenPage';

describe('ForbiddenPage', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    cleanup();
  });

  it('counts down from 3s, then redirects home', () => {
    render(
      <MemoryRouter initialEntries={['/403']}>
        <Routes>
          <Route path="/403" element={<ForbiddenPage />} />
          <Route path="/" element={<div>HOME_LANDING</div>} />
        </Routes>
      </MemoryRouter>,
    );

    screen.getByText('Access Denied');
    expect(screen.getByText(/Returning in 3s/)).toBeTruthy();

    // One act() per second: each flush lets the effect re-arm the next tick,
    // matching how the real timers cascade in the browser.
    for (let step = 0; step < 3; step++) {
      act(() => {
        vi.advanceTimersByTime(1000);
      });
    }

    screen.getByText('HOME_LANDING');
  });
});
