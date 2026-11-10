import { Card } from 'antd';
import { useTranslation } from 'react-i18next';
import type { OverviewSnapshot, ResourceStats } from './api';
import OverviewEmpty from './OverviewEmpty';
import { DatasetIcon, GaugeIcon, JobIcon, ModelIcon } from './icons';

const TILES: Array<{
  field: keyof ResourceStats;
  labelKey: string;
  icon: typeof ModelIcon;
}> = [
  { field: 'models', labelKey: 'stats.models', icon: ModelIcon },
  { field: 'datasets', labelKey: 'stats.datasets', icon: DatasetIcon },
  { field: 'trainingJobs', labelKey: 'stats.trainingJobs', icon: JobIcon },
];

/**
 * 资源水位卡片 (#37): model / dataset / training-job counts. The category
 * tiles always render (the labels are structure); without a snapshot their
 * values show the placeholder dash and the designed empty state explains
 * why.
 */
export default function ResourceStatsCard({ snapshot }: { snapshot: OverviewSnapshot | null }) {
  const { t } = useTranslation('overview');
  const resources = snapshot?.resources ?? null;

  return (
    <Card
      className="overview-card overview-stats-card"
      title={
        <span className="overview-card-title">
          <span className="overview-card-icon">
            <GaugeIcon />
          </span>
          {t('stats.title')}
        </span>
      }
    >
      <div className="overview-stats-tiles">
        {TILES.map(({ field, labelKey, icon: Icon }) => (
          <div key={field} className="overview-stats-tile">
            <span className="overview-stats-tile-icon">
              <Icon />
            </span>
            <span
              className={
                resources
                  ? 'overview-stats-tile-value'
                  : 'overview-stats-tile-value overview-stats-tile-value-empty'
              }
            >
              {resources ? resources[field] : '—'}
            </span>
            <span className="overview-stats-tile-label">{t(labelKey)}</span>
          </div>
        ))}
      </div>
      {!resources && (
        <OverviewEmpty
          icon={<GaugeIcon />}
          title={t('stats.empty')}
          description={t('stats.emptyDesc')}
        />
      )}
    </Card>
  );
}
