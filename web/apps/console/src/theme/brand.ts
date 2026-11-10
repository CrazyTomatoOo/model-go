import type { ThemeConfig } from 'antd';

/**
 * Brand seed tokens — the #28 brand decision (indigo primary, antd geometry).
 * Single source of truth shared by the light theme below and the future dark
 * theme: dark swaps the algorithm, never the seeds (#26 research).
 */
export const brandSeeds = {
  colorPrimary: '#2F54EB',
  colorInfo: '#2F54EB',
  borderRadius: 6,
  fontSize: 14,
  sizeUnit: 4,
  sizeStep: 4,
} as const;

/**
 * Global antd theme.
 *
 * Dark-mode wiring point (#26 research): when the dark theme ships, add
 * `algorithm: theme.darkAlgorithm` here, keyed off the active color scheme;
 * `brandSeeds` above stays shared and no other token changes. Intentionally
 * not shipped in this round (spec #32 Out of Scope).
 *
 * `cssVar: true` lays tokens out as CSS variables (`--ant-*` on :root), which
 * is both the dark wiring surface and what the E2E smoke test asserts against.
 */
export const brandTheme: ThemeConfig = {
  cssVar: true,
  token: { ...brandSeeds },
};
