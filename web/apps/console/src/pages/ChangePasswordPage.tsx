import { Button, Card, Form, Input, Typography } from 'antd';
import { useTranslation } from 'react-i18next';

export default function ChangePasswordPage() {
  const { t } = useTranslation();

  return (
    <Card style={{ maxWidth: 420, margin: '0 auto' }}>
      <Typography.Title level={3}>{t('changePassword.title')}</Typography.Title>
      <Form layout="vertical">
        <Form.Item label={t('changePassword.currentPassword')} name="currentPassword">
          <Input.Password autoComplete="current-password" />
        </Form.Item>
        <Form.Item label={t('changePassword.newPassword')} name="newPassword">
          <Input.Password autoComplete="new-password" />
        </Form.Item>
        <Form.Item label={t('changePassword.confirmPassword')} name="confirmPassword">
          <Input.Password autoComplete="new-password" />
        </Form.Item>
        <Button type="primary" htmlType="submit" block>
          {t('changePassword.submit')}
        </Button>
      </Form>
    </Card>
  );
}
