import { describe, expect, it } from 'vitest';
import { render, waitFor } from '@testing-library/react';
import { Empty } from 'antd';
import i18n from '../i18n';
import LocaleProvider from './LocaleProvider';

function renderEmpty() {
  const { container } = render(
    <LocaleProvider>
      <Empty />
    </LocaleProvider>,
  );
  return container.querySelector('.ant-empty-description');
}

describe('LocaleProvider', () => {
  it('feeds antd components the i18n language, including mid-session switches', async () => {
    await i18n.changeLanguage('zh');
    const description = renderEmpty();
    expect(description?.textContent).toBe('暂无数据');

    await i18n.changeLanguage('en');
    await waitFor(() => expect(description?.textContent).toBe('No data'));
  });
});
