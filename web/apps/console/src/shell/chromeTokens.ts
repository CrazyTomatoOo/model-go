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
} as const;

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
