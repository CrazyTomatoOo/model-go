import { Button, Result } from 'antd';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router';

/** Unknown routes (#34): antd Result 404 with a way back to the console. */
export default function NotFoundPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  return (
    <div
      style={{
        minHeight: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
      }}
    >
      <Result
        status="404"
        title={t('notFound.title')}
        subTitle={t('notFound.subTitle')}
        extra={
          <Button type="primary" onClick={() => navigate('/')}>
            {t('notFound.backHome')}
          </Button>
        }
      />
    </div>
  );
}
