import type { ReactNode } from 'react';

/**
 * Designed empty state (#37): every overview card renders this while the
 * backend has no overview data to serve. Icon stays decorative — the title
 * text carries the meaning for assistive tech.
 */
export default function OverviewEmpty({
  icon,
  title,
  description,
}: {
  icon: ReactNode;
  title: string;
  description: string;
}) {
  return (
    <div className="overview-empty">
      <span className="overview-empty-icon">{icon}</span>
      <span className="overview-empty-title">{title}</span>
      <p className="overview-empty-desc">{description}</p>
    </div>
  );
}
