import { Button, Result } from 'antd';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router';

/** Seconds before the 403 page sends the visitor back to the console home. */
const REDIRECT_SECONDS = 3;

/**
 * No-permission page (#34): explains the denial, counts down, then returns
 * to `/` — the guard chain there resolves to the shell (signed in) or the
 * login bounce (signed out).
 */
export default function ForbiddenPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [secondsLeft, setSecondsLeft] = useState(REDIRECT_SECONDS);

  useEffect(() => {
    if (secondsLeft <= 0) {
      navigate('/', { replace: true });
      return;
    }
    const timer = window.setTimeout(() => {
      setSecondsLeft((left) => left - 1);
    }, 1000);
    return () => window.clearTimeout(timer);
  }, [secondsLeft, navigate]);

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
        status="403"
        title={t('forbidden.title')}
        subTitle={`${t('forbidden.subTitle')} ${t('forbidden.countdown', {
          seconds: secondsLeft,
        })}`}
        extra={
          <Button type="primary" onClick={() => navigate('/', { replace: true })}>
            {t('forbidden.backHome')}
          </Button>
        }
      />
    </div>
  );
}
