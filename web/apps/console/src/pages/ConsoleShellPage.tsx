import { Card, Typography } from 'antd';
import { useTranslation } from 'react-i18next';

export default function ConsoleShellPage() {
  const { t } = useTranslation();

  return (
    <Card>
      <Typography.Title level={3}>{t('console.title')}</Typography.Title>
      <Typography.Paragraph>{t('console.hello')}</Typography.Paragraph>
    </Card>
  );
}
