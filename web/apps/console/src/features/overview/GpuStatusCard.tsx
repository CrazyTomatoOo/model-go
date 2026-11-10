import { Card } from 'antd';
import { useTranslation } from 'react-i18next';
import type { OverviewSnapshot } from './api';
import OverviewEmpty from './OverviewEmpty';
import { GpuIcon } from './icons';

/**
 * GPU 使用卡片 (#37): per-accelerator online/offline status and driver
 * versions. Without an overview API the card renders its designed empty
 * state; with a snapshot it lists one row per accelerator.
 */
export default function GpuStatusCard({ snapshot }: { snapshot: OverviewSnapshot | null }) {
  const { t } = useTranslation('overview');
  const gpus = snapshot?.gpus ?? [];
  const online = gpus.filter((gpu) => gpu.status === 'online').length;

  return (
    <Card
      className="overview-card overview-gpu-card"
      title={
        <span className="overview-card-title">
          <span className="overview-card-icon">
            <GpuIcon />
          </span>
          {t('gpu.title')}
        </span>
      }
      extra={
        gpus.length > 0 ? (
          <span className="overview-gpu-summary">
            {t('gpu.summary', { online, offline: gpus.length - online })}
          </span>
        ) : undefined
      }
    >
      {gpus.length > 0 ? (
        <ul className="overview-gpu-list">
          {gpus.map((gpu) => (
            <li key={gpu.id} className="overview-gpu-item">
              <span className="overview-gpu-dot" data-status={gpu.status} />
              <span className="overview-gpu-name">{gpu.name}</span>
              <span className="overview-gpu-driver">
                {t('gpu.driver', { version: gpu.driverVersion })}
              </span>
            </li>
          ))}
        </ul>
      ) : (
        <OverviewEmpty
          icon={<GpuIcon />}
          title={t('gpu.empty.title')}
          description={t('gpu.empty.desc')}
        />
      )}
    </Card>
  );
}
