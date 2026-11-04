import { Button, Card, Form, Input, Typography } from 'antd';
import { useTranslation } from 'react-i18next';

export default function LoginPage() {
  const { t } = useTranslation();

  return (
    <Card style={{ maxWidth: 420, margin: '0 auto' }}>
      <Typography.Title level={3}>{t('login.title')}</Typography.Title>
      <Form layout="vertical">
        <Form.Item label={t('login.username')} name="username">
          <Input autoComplete="username" />
        </Form.Item>
        <Form.Item label={t('login.password')} name="password">
          <Input.Password autoComplete="current-password" />
        </Form.Item>
        <Button type="primary" htmlType="submit" block>
          {t('login.submit')}
        </Button>
      </Form>
    </Card>
  );
}
