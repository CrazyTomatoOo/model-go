import { Spin } from 'antd';
import './GuardLoading.css';

/**
 * Full-viewport bootstrap loading state for the route guards (#33, checklist
 * A4): replaces the old `return null` so refreshing or deep-linking into any
 * route never flashes a blank white page while auth state is being resolved.
 * The Spin picks up the brand theme from the app-level ConfigProvider.
 */
export default function GuardLoading() {
  return (
    <div className="guard-loading">
      <Spin size="large" />
    </div>
  );
}
