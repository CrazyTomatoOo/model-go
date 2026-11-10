import { Button, Card, Tag } from 'antd';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router';
import { BoltIcon, DatasetIcon, JobIcon, KeyIcon, ModelIcon } from './icons';

type Shortcut = {
  labelKey: string;
  icon: typeof BoltIcon;
  /** Present → the route exists today and the button navigates; absent →
   *  the section ships in a later iteration and the entry stays disabled. */
  to?: string;
};

const SHORTCUTS: Shortcut[] = [
  { labelKey: 'shortcuts.changePassword', icon: KeyIcon, to: '/change-password' },
  { labelKey: 'shortcuts.trainingJobs', icon: JobIcon },
  { labelKey: 'shortcuts.datasets', icon: DatasetIcon },
  { labelKey: 'shortcuts.models', icon: ModelIcon },
];

/**
 * 快捷入口卡片 (#37): one button per console destination. Routes that exist
 * navigate; the upcoming sections (training jobs, datasets, model registry)
 * render disabled with a "coming soon" tag until their pages land.
 */
export default function QuickLinksCard() {
  const { t } = useTranslation('overview');
  const navigate = useNavigate();

  return (
    <Card
      className="overview-card overview-shortcuts-card"
      title={
        <span className="overview-card-title">
          <span className="overview-card-icon">
            <BoltIcon />
          </span>
          {t('shortcuts.title')}
        </span>
      }
    >
      <div className="overview-shortcuts">
        {SHORTCUTS.map(({ labelKey, icon: Icon, to }) =>
          to !== undefined ? (
            <Button
              key={labelKey}
              className="overview-shortcut"
              size="large"
              icon={<Icon />}
              onClick={() => navigate(to)}
            >
              {t(labelKey)}
            </Button>
          ) : (
            <Button key={labelKey} className="overview-shortcut" size="large" icon={<Icon />} disabled>
              {t(labelKey)}
              <Tag className="overview-shortcut-tag">{t('shortcuts.soon')}</Tag>
            </Button>
          ),
        )}
      </div>
    </Card>
  );
}
