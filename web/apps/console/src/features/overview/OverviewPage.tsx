import { Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import { useOverviewSnapshot } from './api';
import GpuStatusCard from './GpuStatusCard';
import QuickLinksCard from './QuickLinksCard';
import ResourceStatsCard from './ResourceStatsCard';
import './OverviewPage.css';

/**
 * 控制台总览 content (#37): GPU status, resource levels, and quick actions.
 * Sits inside the AppShell content area; light-themed on the antd cssVar
 * tokens so the future dark theme (#26/T7) inherits it without edits.
 */
export default function OverviewPage() {
  const { t } = useTranslation();
  const snapshot = useOverviewSnapshot();

  return (
    <div className="overview-page">
      <header className="overview-header">
        <Typography.Title level={3} className="overview-header-title">
          {t('console.title')}
        </Typography.Title>
        <Typography.Paragraph type="secondary" className="overview-header-subtitle">
          {t('console.hello')}
        </Typography.Paragraph>
      </header>
      <div className="overview-grid">
        <GpuStatusCard snapshot={snapshot} />
        <ResourceStatsCard snapshot={snapshot} />
        <QuickLinksCard />
      </div>
    </div>
  );
}
