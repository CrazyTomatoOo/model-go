import type { ReactNode, SVGProps } from 'react';

/**
 * Inline shell icons (#36). @ant-design/icons is not a direct dependency and
 * pnpm isolation keeps it unresolvable from the app, so the chrome ships its
 * own minimal set. All icons inherit `currentColor` and size via `1em`, so
 * chrome styles control color and scale through the font size.
 */

type IconProps = SVGProps<SVGSVGElement>;

function ShellSvg({ children, ...props }: IconProps & { children: ReactNode }) {
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

/** Hamburger — mobile drawer toggle. */
export function MenuIcon(props: IconProps) {
  return (
    <ShellSvg {...props}>
      <path d="M2.5 4.5h11M2.5 8h11M2.5 11.5h11" />
    </ShellSvg>
  );
}

/** Dashboard grid — overview menu entry. */
export function OverviewIcon(props: IconProps) {
  return (
    <ShellSvg {...props}>
      <rect x="2.5" y="2.5" width="4.5" height="4.5" rx="1" />
      <rect x="9" y="2.5" width="4.5" height="4.5" rx="1" />
      <rect x="2.5" y="9" width="4.5" height="4.5" rx="1" />
      <rect x="9" y="9" width="4.5" height="4.5" rx="1" />
    </ShellSvg>
  );
}

/** Key — change-password entries (T3 entry point). */
export function KeyIcon(props: IconProps) {
  return (
    <ShellSvg {...props}>
      <circle cx="5.5" cy="10.5" r="2.75" />
      <path d="M7.7 8.3 13 3M10.5 5.5l2 2M12.5 3.5l1.5 1.5" />
    </ShellSvg>
  );
}

/** Door arrow — logout entry. */
export function LogoutIcon(props: IconProps) {
  return (
    <ShellSvg {...props}>
      <path d="M6 2.5H3.5v11H6" />
      <path d="M9.5 5.5 12.5 8l-3 2.5M12.5 8H6" />
    </ShellSvg>
  );
}
