import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router';
import { afterEach, describe, it } from 'vitest';
import '../i18n';
import NotFoundPage from './NotFoundPage';

function renderAt(path: string) {
  render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path={path} element={<NotFoundPage />} />
        <Route path="/" element={<div>HOME_LANDING</div>} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('NotFoundPage', () => {
  afterEach(cleanup);

  it('renders the 404 copy and navigates home from the action', () => {
    renderAt('/definitely-not-a-route');

    screen.getByText('Page Not Found');
    screen.getByText('The page you visited does not exist or has been removed.');

    fireEvent.click(screen.getByRole('button', { name: 'Back to Home' }));
    screen.getByText('HOME_LANDING');
  });
});
