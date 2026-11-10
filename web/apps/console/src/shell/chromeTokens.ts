import type { ThemeConfig } from 'antd';

/**
 * On-brand chrome ink constants — the single place the shell's surface-on-
 * primary palette lives (#36). The header, sider and mobile drawer are the
 * only dark-on-brand surfaces; page content stays light. When T7 ships the
 * real dark mode, these constants (and the menu tokens below) are its
 * handoff point, together with the `brandSeeds`/darkAlgorithm wiring in
 * theme/brand.ts.
 */
export const chromeInk = {
  /** Primary ink (text, icons) on brand surfaces. */
  base: '#ffffff',
  /** Secondary ink for labels on brand surfaces. */
  dim: 'rgba(255, 255, 255, 0.85)',
  /** Hover wash for interactive chrome elements. */
  hover: 'rgba(255, 255, 255, 0.12)',
  /** Selected/active wash, one step above hover. */
  active: 'rgba(255, 255, 255, 0.22)',
  /** Hairline between chrome regions (sider edge, drawer header/actions). */
  hairline: 'rgba(255, 255, 255, 0.25)',
} as const;

/**
 * The chrome ink palette as CSS custom properties, injected inline by
 * AppShell on every brand-chrome scope (sider, header, mobile drawer). The
 * CSS file consumes var(--shell-ink-*) only — chromeInk above is the single
 * source for the whole palette, which is what the dark-mode wiring (#26)
 * swaps.
 */
export const chromeInkVars: Record<string, string> = {
  '--shell-ink': chromeInk.base,
  '--shell-ink-dim': chromeInk.dim,
  '--shell-ink-hover': chromeInk.hover,
  '--shell-ink-active': chromeInk.active,
  '--shell-hairline': chromeInk.hairline,
};

/**
 * Menu tokens for menus rendered directly on the brand chrome (sider and
 * mobile drawer). Scoped to the shell via a nested ConfigProvider in
 * AppShell, so the global theme and the light page content are untouched.
 */
export const chromeMenuTokens: NonNullable<ThemeConfig['components']>['Menu'] = {
  itemBg: 'transparent',
  subMenuItemBg: 'transparent',
  itemColor: chromeInk.dim,
  itemHoverColor: chromeInk.base,
  itemHoverBg: chromeInk.hover,
  itemSelectedColor: chromeInk.base,
  itemSelectedBg: chromeInk.active,
  itemBorderRadius: 6,
  groupTitleColor: chromeInk.dim,
};
