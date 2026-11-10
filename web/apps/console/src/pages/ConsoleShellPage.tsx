import OverviewPage from '../features/overview/OverviewPage';
import AppShell from '../shell/AppShell';

/**
 * Console overview route (#36): the chrome (brand header, side menu,
 * responsive drawer, user menu) lives in the shared AppShell; T5 (#37)
 * fills the content area with the overview cards.
 */
export default function ConsoleShellPage() {
  return (
    <AppShell>
      <OverviewPage />
    </AppShell>
  );
}
