import type { ReactNode, SVGProps } from 'react';

/**
 * Inline overview content icons (#37), built on the shell icon recipe
 * (src/shell/icons.tsx): 16×16 viewBox, currentColor, sized via 1em so the
 * card styles control color and scale through font size. @ant-design/icons
 * is not a direct dependency and stays unresolvable from the app.
 */

type IconProps = SVGProps<SVGSVGElement>;

function OverviewSvg({ children, ...props }: IconProps & { children: ReactNode }) {
  return (
    <svg
      viewBox="0 0 16 16"
      width="1em"
      height="1em"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.4}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      focusable="false"
      {...props}
    >
      {children}
    </svg>
  );
}

/** GPU chip — accelerator status card. */
export function GpuIcon(props: IconProps) {
  return (
    <OverviewSvg {...props}>
      <rect x="3" y="3" width="10" height="10" rx="1.5" />
      <rect x="6" y="6" width="4" height="4" rx="0.75" />
      <path d="M6 1.5v1.5M10 1.5v1.5M6 13v1.5M10 13v1.5M1.5 6H3M1.5 10H3M13 6h1.5M13 10h1.5" />
    </OverviewSvg>
  );
}

/** Gauge — resource levels card. */
export function GaugeIcon(props: IconProps) {
  return (
    <OverviewSvg {...props}>
      <path d="M2.5 12.5a6.5 6.5 0 1 1 11 0" />
      <path d="M8 9.5 10.8 6" />
      <circle cx="8" cy="9.5" r="1.1" />
    </OverviewSvg>
  );
}

/** Cube — model registry entries. */
export function ModelIcon(props: IconProps) {
  return (
    <OverviewSvg {...props}>
      <path d="M8 2.5 13 5v6l-5 2.5L3 11V5l5-2.5Z" />
      <path d="M3 5l5 2.5L13 5M8 7.5V13" />
    </OverviewSvg>
  );
}

/** Database — dataset entries. */
export function DatasetIcon(props: IconProps) {
  return (
    <OverviewSvg {...props}>
      <ellipse cx="8" cy="4" rx="4.5" ry="1.8" />
      <path d="M3.5 4v8c0 1 2 1.8 4.5 1.8s4.5-.8 4.5-1.8V4" />
      <path d="M3.5 8c0 1 2 1.8 4.5 1.8s4.5-.8 4.5-1.8" />
    </OverviewSvg>
  );
}

/** Flask — training job entries. */
export function JobIcon(props: IconProps) {
  return (
    <OverviewSvg {...props}>
      <path d="M6 2.5h4M6.8 2.5v3L3.6 11.7a1.4 1.4 0 0 0 1.2 2.1h6.4a1.4 1.4 0 0 0 1.2-2.1L9.2 5.5v-3" />
      <path d="M5.4 9.5h5.2" />
    </OverviewSvg>
  );
}

/** Round key — change-password quick action. */
export function KeyIcon(props: IconProps) {
  return (
    <OverviewSvg {...props}>
      <circle cx="5.5" cy="10.5" r="2.75" />
      <path d="M7.8 8.2 13 3M10.6 5.4l2 2" />
    </OverviewSvg>
  );
}

/** Bolt — quick actions card. */
export function BoltIcon(props: IconProps) {
  return (
    <OverviewSvg {...props}>
      <path d="M8.6 2.5 4.6 9h3l-1.2 4.5L10.8 7h-3l0.8-4.5Z" />
    </OverviewSvg>
  );
}
