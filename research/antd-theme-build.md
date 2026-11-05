# antd 5 theming, Vite 7 chunking, and dark-mode-ready tokens

Research ticket: #26. Repo: `CrazyTomatoOo/model-go`. Target: `web/apps/console`.

Version basis (from `web/pnpm-lock.yaml`): antd **5.29.3**, vite **7.3.7**, react **18.3.1**. Every factual claim below links to the primary source that owns it: antd docs and source at the 5.29.3 tag, Vite 7 docs at tag v7.1.0 (identical in the sections used through 7.3.x), Rollup's configuration reference, and the `@ant-design/cssinjs` README.

## 1. antd 5 ConfigProvider theme: what to customize and what it costs

### 1.1 The three token layers

antd 5 organizes design tokens as Seed → Map → Alias, a derivation chain, not three independent groups: "Map Tokens are derived from Seed Tokens, and Alias Tokens are derived from Map Tokens. In most cases, using Seed Tokens is sufficient for custom themes."
Source: [antd 5.29.3 customize-theme, "Design Token"](https://github.com/ant-design/ant-design/blob/5.29.3/docs/react/customize-theme.en-US.md) (published at <https://ant.design/docs/react/customize-theme>).

- **Seed tokens** are "the origin of all design intent"; changing one makes antd's algorithm recompute the related tokens (same doc, "Seed Token").
- **Map tokens** are gradients derived from seeds; they can be overridden individually via `theme.token`, but antd recommends deriving custom ones through `theme.algorithm` so the gradient relationship stays consistent (same doc, "Map Token").
- **Alias tokens** are map-token aliases that batch-control common component styles, e.g. `colorLink` (same doc, "Alias Token").
- The `ConfigProvider` theme API accepts `token: AliasToken`, `algorithm: (token: SeedToken) => MapToken | ((token: SeedToken) => MapToken)[]` (default `defaultAlgorithm`), plus `components` for component tokens and `cssVar` for CSS-variable mode (same doc, "Theme" and "cssVar" API tables).

Conclusion: branding happens at the seed layer; alias/component overrides are for spot fixes.

### 1.2 Which seed tokens to brand for a light theme

The full SeedToken interface: [components/theme/interface/seeds.ts at 5.29.3](https://github.com/ant-design/ant-design/blob/5.29.3/components/theme/interface/seeds.ts). Defaults: [components/theme/themes/seed.ts at 5.29.3](https://github.com/ant-design/ant-design/blob/5.29.3/components/theme/themes/seed.ts).

| Concern | Seed tokens | Default |
| --- | --- | --- |
| Brand color | `colorPrimary`, `colorInfo`, `colorLink`, `colorSuccess`, `colorWarning`, `colorError` | `#1677ff` / `#1677ff` / derived |
| Radius | `borderRadius` | `6` |
| Font scale | `fontSize`, `fontFamily`, `fontFamilyCode` | `14`, system UI stack |
| Spacing grid | `sizeUnit`, `sizeStep` | `4`, `4` |
| Control size | `controlHeight` | `32` |
| Line | `lineWidth`, `lineType` | `1`, `solid` |
| Motion | `motionUnit`, `motionBase`, `motion` | `0.1`, `0`, `true` |
| Neutral bases | `colorBgBase`, `colorTextBase` | `''` (derived) |

Caveat from the source comments: `colorBgBase` and `colorTextBase` exist only to derive background/text gradients ("please do not use this Seed Token directly in the code" — seeds.ts). Brand the primary color, radius, font size and the spacing grid; the rest follows.

What the default algorithm derives automatically (all antd 5.29.3 source):

- Spacing/sizes: `sizeXXL = sizeUnit * (sizeStep + 8)` (48) … `sizeXXS = sizeUnit * (sizeStep - 3)` (4) — [genSizeMapToken.ts](https://github.com/ant-design/ant-design/blob/5.29.3/components/theme/themes/shared/genSizeMapToken.ts)
- Radius: `borderRadiusLG/SM/XS/Outer` computed from `borderRadius` by a staircase (base 6 → LG 8, SM 4, XS 2, Outer 4) — [genRadius.ts](https://github.com/ant-design/ant-design/blob/5.29.3/components/theme/themes/shared/genRadius.ts)
- Font scale: `fontSizeSM/MD/LG/XL`, `fontSizeHeading1…5`, line heights, `fontHeight` all derive from the seed `fontSize` — [genFontMapToken.ts](https://github.com/ant-design/ant-design/blob/5.29.3/components/theme/themes/shared/genFontMapToken.ts)
- Colors: a 10-step palette per seed color is generated with `@ant-design/colors` — [genColorMapToken.ts](https://github.com/ant-design/ant-design/blob/5.29.3/components/theme/themes/shared/genColorMapToken.ts)
- Motion durations and `lineWidthBold = lineWidth + 1` — [genCommonMapToken.ts](https://github.com/ant-design/ant-design/blob/5.29.3/components/theme/themes/shared/genCommonMapToken.ts)

### 1.3 Runtime cost of CSS-in-JS customization on first paint

antd 5 generates styles at runtime through `@ant-design/cssinjs`. The documented mechanism:

- The cache key changes with the theme: "the style may change only if the theme variable is modified, or the version of antd is changed" — [antd blog: Component-level CSS-in-JS](https://github.com/ant-design/ant-design/blob/master/docs/blog/css-in-js.en-US.md) (published at <https://ant.design/docs/blog/css-in-js>).
- The cache is component-level and independent of props: "The same component will only insert styles once, reducing the performance cost of cssinjs during CSS serialization" — [antd blog: Ant Design meets CSS Variables](https://github.com/ant-design/ant-design/blob/master/docs/blog/css-var-plan.en-US.md) (published at <https://ant.design/docs/blog/css-var-plan>).
- Styles are injected into `document.head` by default (`container` option of `StyleProvider`) — [@ant-design/cssinjs README](https://github.com/ant-design/cssinjs).
- antd's own benchmark reports an advantage over emotion/styled for first and second render in the component-level scenario (css-in-js blog, "Benchmark").

What that means concretely: the first render of each used component with a custom theme serializes that component's whole style block once and injects it; later renders hit the hash cache. The cost is one serialization per component × theme, spread across first paint, not per render. Theme switching is the expensive path: "The delay is due to the need for a new round of CSS serialization when switching themes" (css-var-plan blog, "Theme Switching"). Setting `cssVar: true` fixes the hash by mapping tokens to CSS variables, so "switching themes only requires replacing the corresponding CSS variables without going through the lengthy cssinjs serialization process" (css-var-plan blog, "Mapping CSS Variables"; API: `theme.cssVar = { prefix, key }` per the customize-theme doc). Theme isolation selectors use `:where`, which does not raise specificity (`hashPriority: 'low'` default in the cssinjs README).

Practical reading for this console: a static branded light theme costs one style-generation pass per component on first use — small next to network and parse for a login/shell UI; the measurable win from `cssVar: true` appears when dark mode arrives and themes switch at runtime.

## 2. Vite 7 chunk splitting for the antd chunk warning

### 2.1 The documented options

Vite 7 delegates chunking to Rollup: "You can configure how chunks are split using `build.rollupOptions.output.manualChunks`" — [Vite 7 build guide, "Chunking Strategy"](https://github.com/vitejs/vite/blob/v7.1.0/docs/guide/build.md) (published at <https://vite.dev/guide/build>). `build.rollupOptions` is "the same as options that can be exported from a Rollup config file and will be merged with Vite's internal Rollup options" — [Vite 7 build options, `build.rollupOptions`](https://github.com/vitejs/vite/blob/v7.1.0/docs/config/build-options.md) (published at <https://vite.dev/config/build-options>).

The warning threshold is `build.chunkSizeWarningLimit`: type `number`, default `500` kB, "compared against the uncompressed chunk size as the JavaScript size itself is related to the execution time" (build-options doc).

`manualChunks` has two forms — [Rollup `output.manualChunks`](https://rollupjs.org/configuration-options/#output-manualchunks):

- Object form `{ chunkAlias: string[] }`: "easier and safer"; each chunk contains the listed modules and all their dependencies "unless they are already in another manual chunk".
- Function form `(id, { getModuleInfo, getModuleIds }) => string | void`: each resolved module id is passed in; returning a string puts that module and its dependencies into the named chunk (the doc's example returns `'vendor'` for every id containing `node_modules`).
- Both forms merge aggressively: e.g. `manualChunks: { lodash: ['lodash'] }` merges all lodash modules "even if you are only using imports of the form `import get from 'lodash/get'`".

Options ranked for this app:

1. **`manualChunks` (object form)** — split the single warning-sized chunk into named vendors. Does not reduce total bytes; shrinks the largest chunk under the warning and lets browsers cache vendors independently. No app-code changes.
2. **On-demand import via dynamic import** — Vite turns every dynamic import into its own chunk and emits `vite:preloadError` when one fails to load ([build guide, "Load Error Handling"](https://github.com/vitejs/vite/blob/v7.1.0/docs/guide/build.md)); [dynamic import is a first-class feature](https://github.com/vitejs/vite/blob/v7.1.0/docs/guide/features.md). With `React.lazy`/route-level splits, each page becomes a chunk and its antd components are only fetched when visited. Requires routing changes in `web/apps/console/src/App.tsx`.
3. **Tree-shaking (already active)** — "antd supports tree shaking of ES modules, so using `import { Button } from 'antd';` would drop js code you didn't use" — [antd 5.29.3 getting-started, "Import on Demand"](https://github.com/ant-design/ant-design/blob/5.29.3/docs/react/getting-started.en-US.md). antd's CSS-in-JS "implemented the ability to selectively import CSS without relying on babel-plugin-import for automatic tree-shaking" (css-var-plan blog). No `babel-plugin-import` is needed.
4. **Raising `chunkSizeWarningLimit`** — silences the warning, changes no payload. Acceptable only after an intentional split is verified; note in the config why.

### 2.2 Expected size reduction, stated honestly

Chunk splitting moves bytes, it does not delete them. The ~790 kB chunk named in the ticket becomes, with the draft below, `vendor-react` (react + react-dom), `vendor-antd` (antd plus its transitive rc-* / `@ant-design/*` / dayjs, pulled in by the dependency rule), `vendor-router`, and the app chunk. Total JS is unchanged; the largest chunk shrinks and the vendors cache across deploys. Real byte reduction comes from tree-shaking (already active) and route-level dynamic imports (pages fetched on navigation). Exact numbers were not measured: `web/node_modules` is absent in this checkout, the ticket forbids `pnpm install`, and no build was run. The ~790 kB figure is the ticket's build warning, treated as given context, not reproduced here.

### 2.3 Interaction with antd v5 CSS-in-JS

No build-time interaction exists: antd does not emit CSS chunks; it serializes styles and injects them into `document.head` at runtime (cssinjs README; css-var-plan blog). Chunk boundaries therefore do not affect theme hashing or style injection, and no extra Vite config is needed for antd's CSS-in-JS. Two consequences worth knowing:

- Styles are generated when a component first renders, so a route split that lazy-loads antd components delays their one-time style generation until that chunk evaluates — same per-component cost, later in the timeline.
- Theme isolation relies on class hashes derived from token values and the antd version, not on module identity (css-in-js blog), so manual chunks cannot break it.

## 3. Making dark mode cheap later

Keep brand seeds identical across modes; switch only the algorithm.

- antd ships `theme.defaultAlgorithm`, `theme.darkAlgorithm`, `theme.compactAlgorithm`; algorithms "can be used alone or in any combination, for example, dark and compact algorithms can be combined" — customize-theme doc, "Use Preset Algorithms" and "Algorithm".
- The dark algorithm is a derivative `(SeedToken, MapToken?) => MapToken` that expands the same seeds with dark palettes via `generate(token[color], { theme: 'dark' })` — [components/theme/themes/dark/index.ts at 5.29.3](https://github.com/ant-design/ant-design/blob/5.29.3/components/theme/themes/dark/index.ts). Same seeds in, mode-appropriate maps out.
- Dynamic switching needs no extra setup: "you can dynamically switch themes at any time through the `theme` property of `ConfigProvider` without any additional configuration" (customize-theme, "Switch Themes Dynamically"); the official demo holds the config in React state and re-renders `ConfigProvider theme={{ token: { colorPrimary } }}` — [dynamic-theme.tsx](https://github.com/ant-design/ant-design/blob/master/docs/react/_demo/dynamic-theme.tsx).
- Nested `ConfigProvider`s inherit every token the child does not override — customize-theme, "Nested Theme".
- Pass `{}`, never `undefined`, when you mean "no override": `theme={undefined}` adds/removes the `DesignTokenContext` provider and remounts the subtree (customize-theme FAQ).
- Consume tokens in app code with `theme.useToken()` or `theme.getDesignToken(config)` — customize-theme, "Consume Design Token".
- Set `cssVar: true` so the eventual mode switch replaces CSS variables instead of re-serializing styles (Section 1.3; css-var-plan blog).

With the skeleton below, dark mode later is one expression: `theme={mode === 'dark' ? darkTheme : lightTheme}`. If a specific neutral must be pinned, override `colorBgBase`/`colorTextBase` inside `darkTheme.token` only — everything else stays shared.

## 4. Token skeleton (directly usable)

```ts
// web/apps/console/src/theme/brand.ts
import type { ThemeConfig } from 'antd';
import { theme } from 'antd';

/**
 * Brand seeds, identical in every mode. The algorithm turns them into
 * map/alias tokens, so light and dark stay consistent by construction.
 * Replace the brand color; the rest mirror antd 5 defaults.
 */
const brandSeeds: ThemeConfig['token'] = {
  colorPrimary: '#1677ff', // TODO: brand color
  colorInfo: '#1677ff',
  colorLink: '', // empty = derived from colorPrimary
  borderRadius: 6,
  fontSize: 14,
  fontFamily: `-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial,
'Noto Sans', sans-serif, 'Apple Color Emoji', 'Segoe UI Emoji', 'Segoe UI Symbol',
'Noto Color Emoji'`,
  fontFamilyCode: `'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, Courier, monospace`,
  sizeUnit: 4, // spacing grid: sizeXXL = 4 * (4 + 8) = 48 ... sizeXXS = 4
  sizeStep: 4,
  controlHeight: 32,
  // motion / line seeds left at antd defaults
};

export const lightTheme: ThemeConfig = {
  token: brandSeeds,
  algorithm: theme.defaultAlgorithm,
  // Stable hash: a future theme switch swaps CSS variables instead of re-serializing.
  cssVar: true,
};

// Out of scope today; pre-wired so a mode toggle only swaps this object.
export const darkTheme: ThemeConfig = {
  token: brandSeeds, // darkAlgorithm derives backgrounds/text/palettes from the same seeds
  algorithm: theme.darkAlgorithm,
  cssVar: true,
};
```

The console already has a single `ConfigProvider` in `web/apps/console/src/antd/LocaleProvider.tsx`; the theme belongs there:

```tsx
// web/apps/console/src/antd/LocaleProvider.tsx (edited excerpt)
import { ConfigProvider } from 'antd';
import { lightTheme } from '../theme/brand';

export default function LocaleProvider({ children }: { children: ReactNode }) {
  const { i18n } = useTranslation();
  const locale = locales[i18n.language] ?? enUS;
  return (
    <ConfigProvider locale={locale} theme={lightTheme}>
      {children}
    </ConfigProvider>
  );
}
```

Caveats that belong with the snippet:

- `ConfigProvider` does not reach static methods (`message.xxx`, `Modal.xxx`, `notification.xxx`); use the `App` component or `Modal.useModal` so those calls see context (customize-theme doc warning).
- Component-specific tweaks go in `theme.components` (e.g. `Button: { borderRadius: 4 }`), not in `token`, to stay scoped (customize-theme, "Customize Component Token").
- `cssVar: true` routes token values through CSS variables; code that needs current values should read `theme.useToken()` rather than computed styles (css-var-plan blog; customize-theme cssVar API).

## 5. Vite chunk config draft

Merge into the existing [`web/apps/console/vite.config.ts`](file:///Volumes/work/Project/Golang/src/model-go/web/apps/console/vite.config.ts), which currently defines only `plugins`, `server` and `test`:

```ts
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vitest/config';

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/api': { target: 'http://127.0.0.1:8080', changeOrigin: true },
    },
  },
  test: { environment: 'jsdom', globals: false },
  build: {
    rollupOptions: {
      output: {
        manualChunks: {
          // Object form: listed modules + their dependencies (rc-*, @ant-design/*,
          // dayjs) land here unless already assigned to another manual chunk.
          'vendor-react': ['react', 'react-dom'],
          'vendor-antd': ['antd'],
          'vendor-router': ['react-router'],
        },
      },
    },
  },
});
```

Why this shape:

- `'vendor-antd': ['antd']` pulls antd's transitive dependencies into the same chunk by Rollup's dependency rule; antd + rc-* + `@ant-design/cssinjs` form one cacheable unit.
- `'vendor-react'` separates the React runtime so upgrading antd does not invalidate the React chunk and vice versa.
- Importing antd from the barrel stays tree-shaken by Vite/Rollup (getting-started doc), so `vendor-antd` contains only the components the console imports.
- If `vendor-antd` still exceeds the 500 kB default after the split, the next lever is route-level dynamic imports, not `chunkSizeWarningLimit`. The default threshold compares uncompressed size (build-options doc) and is a heuristic; document any deliberate bump.

Expected outcome: the single ~790 kB chunk becomes three vendor chunks plus app chunks; the warning either clears (if the antd-only code is under 500 kB) or narrows to `vendor-antd`; total bytes are unchanged; vendors cache across deploys. Exact figures need a build, which was out of scope here.

## Sources

- antd 5.29.3 customize-theme: <https://github.com/ant-design/ant-design/blob/5.29.3/docs/react/customize-theme.en-US.md> (published: <https://ant.design/docs/react/customize-theme>)
- antd 5.29.3 seed interface: <https://github.com/ant-design/ant-design/blob/5.29.3/components/theme/interface/seeds.ts>
- antd 5.29.3 seed defaults: <https://github.com/ant-design/ant-design/blob/5.29.3/components/theme/themes/seed.ts>
- antd 5.29.3 map-token derivations: <https://github.com/ant-design/ant-design/tree/5.29.3/components/theme/themes/shared> (genSizeMapToken, genRadius, genFontMapToken, genColorMapToken, genCommonMapToken)
- antd 5.29.3 dark algorithm: <https://github.com/ant-design/ant-design/blob/5.29.3/components/theme/themes/dark/index.ts>
- antd 5.29.3 getting-started (tree shaking): <https://github.com/ant-design/ant-design/blob/5.29.3/docs/react/getting-started.en-US.md>
- antd blog, Component-level CSS-in-JS: <https://github.com/ant-design/ant-design/blob/master/docs/blog/css-in-js.en-US.md> (published: <https://ant.design/docs/blog/css-in-js>)
- antd blog, Ant Design meets CSS Variables: <https://github.com/ant-design/ant-design/blob/master/docs/blog/css-var-plan.en-US.md> (published: <https://ant.design/docs/blog/css-var-plan>)
- @ant-design/cssinjs README: <https://github.com/ant-design/cssinjs>
- Vite 7 build guide: <https://github.com/vitejs/vite/blob/v7.1.0/docs/guide/build.md> (published: <https://vite.dev/guide/build>)
- Vite 7 build options: <https://github.com/vitejs/vite/blob/v7.1.0/docs/config/build-options.md> (published: <https://vite.dev/config/build-options>)
- Vite 7 features, dynamic import: <https://github.com/vitejs/vite/blob/v7.1.0/docs/guide/features.md> (published: <https://vite.dev/guide/features#dynamic-import>)
- Rollup `output.manualChunks`: <https://rollupjs.org/configuration-options/#output-manualchunks>

## Verification limits

- `web/node_modules` does not exist in this checkout. Per the ticket, `pnpm install` was forbidden and the console build was skipped, so chunk sizes above are structural expectations; the ~790 kB figure is the ticket's build warning.
- The antd docs and source citations use the 5.29.3 tag to match the lockfile; the blog posts live only on `master` but describe the v5 CSS-in-JS architecture cited here.
