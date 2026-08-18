# demo-dog 前端生产就绪度评估报告

> 评估对象：`/Volumes/E/JYW/创意项目/demo-dog/frontend/`
> 评估依据：实际阅读源码（约 5800 行 TypeScript/TSX + 配置），不依赖推测。
> 仅列出代码中**实际存在**的事实，并按 P0/P1/P2 标注严重程度。

---

## 0. 整体概览（项目画像）

- 技术栈：React 18.3 + Vite 5.4 + TypeScript 5.7（strict）+ Tailwind 3.4，**纯 CSR SPA**。
- 路由：`src/lib/router.ts` 自实现的 hash 路由器（`#/page?...`），无 react-router。
- 数据层：`src/lib/api.ts` 的 `fetch` + `src/lib/ws.ts` 单一共享 WebSocket；**无 React Query / SWR / Zustand / Redux**。
- 图表/可视化：图表 100% 手写 SVG（`TimeSeriesChart`、`TraceWaterfall`、`Sparkline`、`ServiceMapGraph`）。
- 状态管理：纯 React `useState` + `useRef`，URL hash 通过 `src/hooks/useHashState.ts` 反射。
- 构建产物大小（`dist/assets/index-1lKPdMxl.js`）：**约 283 KB（gzip 前）**，单 chunk 全量打入。
- 依赖数（`package.json`）：运行时仅 `react`、`react-dom` 两个；dev 依赖 9 个。
- 没有 ESLint / Prettier / Husky / Storybook / Cypress / Vitest / Playwright 任何一种工程脚手架。
- 没有 `.github/workflows/`、没有 CI 配置文件。
- 没有 `README.md`、`CONTRIBUTING.md`、`STYLEGUIDE.md`、`docs/`。
- 全文件未发现任何单元测试目录、文件或调用。

---

## 1. 身份认证 & 用户管理（Authentication & User Management）

### ✅ 已具备
- 无。前端**完全没有任何登录、SSO、OAuth、用户、角色或权限**概念。

### ⚠️ 半成品 / 临时做法
- 无。`vite.config.ts` 把 `/api` 直接代理到 `http://localhost:18080`，没有任何 `Authorization` 头注入或 token 拦截。
- 所有页面均假设已登录用户身份，直接读取 `service`、`service-detail`、`ingest/otlp` 等只读/写接口。
- `App.tsx` 第 31–54 行的键盘快捷键 `g+l/m/t/o/e/d/s/v` 已经能驱动到任何页面，连登录页都**不存在**。

### ❌ 缺失
| 严重度 | 缺口 |
|---|---|
| **P0** | 无登录页、无注册、无会话到期处理。任何能访问 `:5173` 的人都能**直接写入** `POST /api/ingest/otlp`、`/api/ingest/otlp-json`、`/api/seed`，等于门户大开。 |
| **P0** | `src/lib/api.ts` 的 `getJson/postJson` 没有 `Authorization` 头或 `credentials`/cookie 注入逻辑，也没有拦截 401/403。 |
| **P0** | 后端 `/metrics`、`/api/health`、`/api/*` 都未做权限分层；前端也未做 RBAC，至少要挡住 `ingest`、`seed`、`snapshots`、`recentPayloads` 这类可写/可嗅探的端点对未登录用户暴露。 |
| **P1** | 不存在 SSO（OIDC/SAML/OAuth）接入；企级生产几乎必然要做企业 IdP 对接。 |
| **P1** | 不存在多租户/多角色（admin / viewer / editor）分离 UI；`Sidebar.tsx`、`IngestDemo.tsx` 等敏感操作页对所有用户都可见。 |
| **P2** | 没有"记住我"、token 自动刷新、设备列表、登录审计。 |

---

## 2. 状态管理（State Management）

### ✅ 已具备
- 数据形状完整：`src/types/api.ts` 248 行，几乎所有后端类型都有对应 TS 接口。
- `src/lib/ws.ts` 正确实现了**单一共享** WebSocket（`getSharedStreamClient()`），并自带指数退避重连（`scheduleReconnect` 使用 `2^min(retries,5)` 倍数）。
- `src/components/ErrorBoundary.tsx` 是一个标准的 `getDerivedStateFromError` 错误边界，并由 `App.tsx` 用 `key={safePage}` 包住每个页面，**切换路由会卸载整页子树以隔离崩溃**——这点比裸 React 错误边界成熟很多。
- `src/hooks/useHashState.ts` 提供 `useHashState`/`useHashStateBool`/`useHashStateJson`，状态序列化进 URL 自带 `replaceState` + `dispatchEvent` 兜底（注释明确写"不污染 history 栈"）。
- `src/components/Toast.tsx` 用模块级 `listeners` + `nextId` 实现订阅式 toast。
- `src/lib/api.ts` 第 130–135 行：`serviceDetail` 拒绝空字符串调用，把防御写进了 API 边界——这是少见的好习惯。

### ⚠️ 半成品 / 临时做法
- 数据获取**全部裸 `fetch` + `useEffect`**：例如 `Overview.tsx`、`Traces.tsx`、`Metrics.tsx`、`ServiceDetailPage.tsx`、`Logs.tsx` 都用 `setInterval(..., 4000~8000ms)` 轮询；一旦切页、卸载会出现：
  - **没有乐观更新**（`Logs.tsx` 的 live tail 是"先 push 后过滤"，不算乐观写）。
  - **没有请求去重 / 缓存**（切到 Logs 再切走再切回来，会立刻重新打同一条 `/api/services`）。
  - **没有请求取消**（`AbortController` 在整个 `src/` 零命中，`grep "AbortController"` 返回空）。
  - **没有 staleness/后台节流**（`setInterval` 持续打，即使页签被切到后台）。
- "重试"仅有 WS 的退避重连，HTTP 请求**没有重试也没有错误退避**；一旦 `/api` 抖动，UI 直接弹红条。
- 错误显示仅 `ErrorBox`（`src/components/Feedback.tsx`），没有 toast 化、网络错误专属化。
- `Sidebar.tsx` 内有两个并行 `useEffect` 跑 `services()` + `qps()` 轮询，没有共享请求。

### ❌ 缺失
| 严重度 | 缺口 |
|---|---|
| **P0** | 没有 Query 层（React Query / SWR / RTK Query），导致页面间无法复用缓存、`staleTime`、`placeholderData`、聚焦重取等全部手写。 |
| **P0** | 没有 `AbortController`，用户切走页面时，请求仍然会被飞行中完成并触发 `setState`，**已卸载组件 warning 与潜在内存泄漏**。 |
| **P1** | 没有乐观更新框架：例如 `IngestDemo.tsx`、`ServiceDetailPage.tsx` 的"seed"/"navigate" 操作都没有 rollback 机制。 |
| **P1** | 没有全局 server-state store：`services`、`health`、`metricNames` 三个数据在 `Overview.tsx`、`Logs.tsx`、`Metrics.tsx`、`Traces.tsx`、`Sidebar.tsx`、`TopBar.tsx` 都各自 fetch 一份（最少 5 处独立 `useEffect` 拉 `/api/services`）。 |
| **P2** | 没有 `error.tsx` / `loading.tsx` / `not-found.tsx` 等分层错误页。仅有一个 `ErrorBoundary` 红色框 + 路由无效时回落到 `overview`（`App.tsx` 第 60 行）。 |
| **P2** | `App.tsx` 全局键盘 `g + l/m/t/o/...` 监听没有 throttle，连续按会被 `setTimeout(removeEventListener, 1000)` 错位重置（见 `App.tsx` 第 70–95 行）。 |

---

## 3. 性能（Performance）

### ✅ 已具备
- 依赖很轻（运行时只有 React + ReactDOM），无重型第三方。
- `src/components/charts/TimeSeriesChart.tsx`、`TraceWaterfall.tsx`、`Sparkline.tsx`、`Sidebar.tsx` 的 `ServiceMapGraph` 都是**自写 SVG**，避免引入 d3 / recharts / chart.js 等会膨胀 bundle 的库。
- `Overview.tsx`、`Metrics.tsx` 大量使用 `useMemo`、`useCallback` 派生 series / 过滤数组（共 35 个 perf 标记命中，含 useMemo/useCallback）。
- `Vite` 的生产构建配置简洁，alias `@ → src` 已开。

### ⚠️ 半成品 / 临时做法
- `dist/assets/index-1lKPdMxl.js` ≈ **283 KB 整个单 chunk**：所有页面、所有动画组件、所有图表全部塞进一个 bundle，没有代码分割。
- 没有路由级 `lazy()` / `Suspense`：`grep "React.lazy|Suspense|import()"` 全部零命中。首屏即加载 ~283 KB。
- 没有列表虚拟化：`grep "react-window|@tanstack/react-virtual|tanstack/react-virtual"` 全空。`Logs.tsx` 用 `rows.slice(0,300)`（常量第 285 行），超过 300 行直接**截断丢弃**不渲染，也不是虚拟滚动。
- `Sidebar.tsx` 内 `setInterval` 同时启 2 个（4s + 5s），且每 4s 把 `qpsByService` **整体重写**——超过 100 个服务后会出现明显抖动。
- `Overview.tsx` 每 4s 把整个 services 数组 + QPS 全部 `setState`，重渲染全部 `<StatCard>`、`<TimeSeriesChart>` 与表格。
- `Logs.tsx` 的 `liveEvents` 用 `useStream({ kinds: ["log"], max: 50 })`，每条事件都进 ring buffer 然后 `setEvents`；虽然有 `bufferMs` 节流，但 buffer 默认 `0`（第 17 行），所以**每条 WS 消息都触发一次 setState**——万级 ingress 直接卡爆主线程。
- `Logs.tsx` `useStream({ kinds: ["log"], max: 50 })` 这个 50 上限容易丢消息，特别是在服务重启流量峰值期。
- 没有 `React.memo` 包裹任何子组件（`grep "React.memo"` 零命中）；`StatCard`、`SeverityBadge`、`Sparkline` 这些频繁用到的组件无记忆化。
- `Sidebar.tsx` 的 `qpsByService` `prev => ({...})` 写法只要任一服务名变化就会重建整个对象，触发所有 `ServiceRow` 重渲染。
- `Magnetic` 组件（`anim/Magnetic.tsx`）在 `ServiceDetailPage.tsx` KPI 上**用 5 个 Magnetic 嵌套**监听 mousemove，即使没有动画也持续运行（reduced-motion 检查后 `enabled` 仍依赖 ref）；高频 setState。
- `TraceWaterfall.tsx` 使用递归 `findDepth` 且每次都遍历 `valid.find(...)`，对 100+ span 的 trace 是 O(N²)。

### ❌ 缺失
| 严重度 | 缺口 |
|---|---|
| **P0** | 无路由级 code-splitting。冷启动 ~283 KB，所有页面一次性下载；移动端首屏 LCP 必然劣化。 |
| **P0** | 无虚拟滚动。`Logs.tsx` 限死 300 行硬切；`Sidebar.tsx` 全部服务一次性渲染。10w 行日志直接白屏。 |
| **P1** | `useStream` 默认 `bufferMs=0`，WS 高吞吐下会卡死主线程，需要可配置 throttle。 |
| **P1** | 无 `React.memo`，重渲染粒度过粗。 |
| **P1** | WebSocket 在用户切到后台标签页后仍持续推送 + setInterval 轮询；需要 `document.visibilityState` 暂停。 |
| **P2** | 没有 `RequestIdleCallback` / `useDeferredValue` 等放弃式渲染。 |
| **P2** | 没有 bundle analyzer、size-limit、source-map 关闭等打包后优化。 |
| **P2** | 字体 Inter / JetBrains Mono 仅在 `tailwind.config.js` 引用而 `index.html` 没 `<link>` 引入，靠系统降级（见 `tailwind.config.js` 第 21–25 行 + `index.html`），初次访问实际无网络字体。 |

---

## 4. 无障碍（Accessibility, a11y）

### ✅ 已具备
- `index.html` 设置了 `lang="en"`（注意：i18n 缺失状态下是 bug）。
- `Body` 有 `bg-grafana-bg text-grafana-text` 颜色对比基础类。
- `SearchBox.tsx` 的清空按钮有 `aria-label="Clear search"`（仅此一处）。
- `BarLoader` 有 `aria-label="loading chart"`，其余动画装饰（`Pulse / LiveBadge / Glitch / ShinyText / TrendArrow`）有 `aria-hidden="true"`（共 7 处命中）。
- `TopBar.tsx` 第 195 行的状态徽章（健康/WS/全局 err）使用 `title=` 提示。

### ⚠️ 半成品 / 临时做法
- 深色主题 `#0b0d12` 与 `#9fa6b2`（muted）对比度大致接近 WCAG AA 边缘，需要审计；`#d8d9da on #181c22` 在小字号（10–11px）多处不达标。
- 多处表格的 `<th>` 没有 `scope` 属性（`Overview.tsx`、`Metrics.tsx`、`Traces.tsx`、`Dashboards.tsx` 的 `PanelRenderer` 等）。
- 没有 `role="banner"`、`role="navigation"`、`role="main"`、`role="complementary"` 等地标语义。`<aside>` 用了（`Sidebar.tsx`），但 `<header className="header">` 与 `<main>` 没补 ARIA。
- `Sidebar.tsx` 的 11 个导航项是 `<button>`，但没有 `aria-current="page"` 标记当前页；只用 className 高亮（视觉用户可见，屏幕阅读器听不到）。
- 多处仅 `<button>` / `<input>` 但缺 `<label>`：`Overview.tsx` 时间挑选、Dashboards 右侧刷新、Logs 的 `service` 输入框都没有 `htmlFor`/`aria-label`。
- 自定义下拉（`TimeRangePicker.tsx` 第 28–45 行）没有 `aria-expanded` / `role="menu"`，键鼠不可达。
- 模态 `CommandPalette.tsx` 没有 `role="dialog" aria-modal="true"`、`aria-labelledby`；焦点不会 trap（`Esc` 关闭但不是真正的 dialog 模式）。
- `ErrorBoundary` 错误回退没有 `role="alert"`。
- `Toast.tsx` 没有 `role="status" aria-live="polite"`。
- `Blink/Glitch`/`GradientText` 用 `mix-blend-mode: screen`，对前庭/光敏感人群是已知问题；虽有 `prefers-reduced-motion` 兜底（`GradientText`、`Magnetic`、`CountUp`），但 `BlurText`、`Stagger`、`Pulse`、`LiveBadge`、`TrendArrow` **不查 prefers-reduced-motion**。
- 颜色唯一区分的"严重"通道（`SeverityBadge` `COLORS`）：`TRACE` 与 `DEBUG` 颜色相同（`#6b7280`），仅靠文字大小区分——色盲与单色访问者无法识别。
- 大量 emoji `⚠ ⏱ ⎘ ⬇ ⟳ ⚡ ☰ ⌘` 直接用作文案 / 按钮，屏幕阅读器会逐字念出。

### ❌ 缺失
| 严重度 | 缺口 |
|---|---|
| **P0** | 页面无键盘 Tab 聚焦样式（Tailwind 没有全局 `focus-visible` ring；`:focus-visible` 仅在 `SearchBox` / Inputs 上零散出现）。 |
| **P0** | `Logs.tsx`、`Traces.tsx` 的列表用 `index` 做 key（`key={i}`），屏幕阅读器列表语义错乱。 |
| **P1** | 没有 Skip-to-main-content 链接；侧栏 Tab 走 60 个 NavItem 才能进入内容。 |
| **P1** | `CommandPalette` 没焦点 trap、没 aria 属性，违背 WCAG 2.1 模态要求。 |
| **P1** | `SeverityBadge` 仅用颜色传达严重级（红/黄/绿），违反 WCAG 1.4.1。 |
| **P1** | 缺乏对 `prefers-reduced-motion` 的统一处理（`BlurText`、`Stagger`、`Pulse`、`LiveBadge`、`TrendArrow`、`BarLoader`、`Glitch`、`TrueFocus`、`ShinyText` 都未查询 media query）。 |
| **P2** | 缺少 Lighthouse / axe 自动化扫描脚本。 |

---

## 5. 国际化（i18n）

### ✅ 已具备
- **没有**任何 i18n 库（`grep "react-i18next|i18next|Lingui"` 全空）。
- **没有**任何语言包目录（`src/locales/`、`src/i18n/` 不存在）。
- 所有页面文案**全部硬编码英文**：`Sidebar.tsx`、`TopBar.tsx`、`Logs.tsx` 等。

### ⚠️ 半成品 / 临时做法
- `index.html` `<html lang="en">` 永远是英文——切换语言无从谈起。
- `relativeTime` / `fmtTime` / `duration`（`src/lib/time.ts`）全部依赖 `Date.toLocaleTimeString(undefined, ...)`，`undefined` 会拿浏览器 locale；没有显式 `Intl.DateTimeFormat` 注入。
- 数字格式化 `toFixed`、`toLocaleString` 默认走浏览器 locale。

### ❌ 缺失
| 严重度 | 缺口 |
|---|---|
| **P0** | 整体 i18n 框架全无：文案、中文/英文/其他语种切换、复数规则、日期时间数字 locale、RTL、ICU MessageFormat 都不存在。 |
| **P1** | 即便只是 demo，中文 dashboard 文案（"调用链"、"日志"、"指标"）都是企级内部演示硬需求；目前出口直接是英文。 |
| **P2** | `TopBar.tsx` 的 `now.toLocaleTimeString()` 不传 locale，应该显式传 `Intl.DateTimeFormat(locale, …)`。 |

---

## 6. 响应式设计（Responsive Design）

### ✅ 已具备
- Tailwind 栅格（`md:`、`lg:`、`xl:`）使用广泛：`Overview.tsx` `grid-cols-1 md:grid-cols-4`、`Metrics.tsx` `grid-cols-2 md:grid-cols-4`、`Traces.tsx` `grid-cols-12 gap-3 lg:col-span-4` 等。
- 多数图表 `width: 100%` + `max-width: 100%` 跟随容器。
- `Cmd/Ctrl+K` 命令面板固定宽度 640px + `max-w-[92vw]`。

### ⚠️ 半成品 / 临时做法
- **Sidebar 是固定 240 px（`Sidebar.tsx` 第 173 行 `w-60 shrink-0`），不折叠**：手机宽度直接挤压主区到几乎没有。
- 主区仅靠 `min-w-0` 避免溢出，没有汉堡菜单；移动端体验几乎不可用。
- `TraceWaterfall.tsx` 的左边文字宽度固定 200px，第 102 行 `style={{ width: 200, paddingLeft: indent }}` —— 窄屏溢出。
- `TopBar.tsx` 的横向徽章链（health + ws + tiers + svc + L/M/S + queries + clock）不会折叠；在 < 1024px 直接横向溢出。
- `Logs.tsx` 的 toolbar 用了 `flex flex-wrap`，是少数有 wrap 的位置（行 295）。
- `Table.tsx` 没有 sticky column、移动端可滚动列；表格在小屏只能横向溢出整个页面。
- 没有 viewport meta 之外的任何 mobile optimization：`viewport=width=device-width, initial-scale=1.0`，无 theme-color、无 apple-touch-icon。
- 没有任何触屏手势 / 触屏交互测试——`Magnetic` 在 `Magnetic.tsx` 第 41 行 `:React.MouseEvent` 只对鼠标反应；桌面独有、移动不存在。

### ❌ 缺失
| 严重度 | 缺口 |
|---|---|
| **P0** | 无移动端/平板适配；`Sidebar` 固定 240 px 不折叠，触控屏、SM/MD 屏幕**无法正常使用**。 |
| **P1** | 无断点策略文档；当前 `md:`(768) 与 `lg:`(1024) 是 Tailwind 默认值，没有自定义 token。 |
| **P1** | `Table` 横滚方案缺失；超过 6 列的表（小屏）一定坏布局。 |
| **P2** | 没有 `@media (pointer: coarse)` 触屏分支：所有磁吸/hover 仅对鼠标设备友好。 |
| **P2** | 没有 orientation 锁定 / 横屏布局评估。 |

---

## 7. 测试（Testing）

### ✅ 已具备
- **无**。全工程没有任何 `*.test.*`、`*.spec.*`、`__tests__`、`tests/` 目录。

### ⚠️ 半成品 / 临时做法
- `package.json` 第 7–10 行只定义了 `dev / build / preview / typecheck`，没有任何 `test` script。
- 没有任何 Vitest / Jest / Playwright / Cypress / Testing Library 依赖。
- `Logs.tsx` 的 `parseSearchQuery` 函数（第 43–110 行）逻辑复杂（`service=`, `severity>=WARN`, `host=x`, 引号配对等），却没有任何单测覆盖。
- `useHashState` / `useStream` / `StreamClient.scheduleReconnect` 是逻辑核心，也无单测。
- 路径黑魔法：`api.ts` 第 130 行 `serviceDetail` 对空字符串的特殊处理是为防 `GET /api/services//detail` 404（见注释），这种边界 case 必须有测试，但实际没有。

### ❌ 缺失
| 严重度 | 缺口 |
|---|---|
| **P0** | 单元测试为 0；合规/SOX/审计场景中这是硬伤。 |
| **P0** | 集成测试为 0；没有任何 mock server / contract test。 |
| **P0** | E2E 测试为 0；登录/种子/查询/钻取的关键路径都没有自动化覆盖。 |
| **P1** | 没有覆盖率门槛与报告生成。 |
| **P2** | 没有 Storybook + interaction tests。 |

---

## 8. CI/CD & 构建（CI/CD & Build）

### ✅ 已具备
- `package.json` 有 4 个 npm scripts：`dev`、`build (tsc + vite build)`、`preview`、`typecheck`。
- `tsconfig.json` 启用了 `"strict": true`、`noFallthroughCasesInSwitch`、`isolatedModules`、`useDefineForClassFields` 等较为严格的 TS 选项。
- `vite.config.ts` 配置了 React plugin、path alias、dev server proxy `/api`（含 `ws: true`）、端口 5173。
- `tailwind.config.js` 配 Grafana-like 调色板与字体策略。
- `tsconfig.node.json` 单独为 `vite.config.ts` 开了 composite 配置。

### ⚠️ 半成品 / 临时做法
- `package.json` 完全缺失：
  - `lint` / `lint:fix` 脚本；
  - `format` 脚本；
  - `test` / `test:e2e` / `test:coverage` 脚本；
  - `prepare` / `husky` 钩子；
  - `release` / `version` / `changelog` 脚本。
- 没有 `.eslintrc*`、`eslint.config.*`、`.prettierrc*`、`.editorconfig` 任何一种。
- 没有 `.github/workflows/`、`gitlab-ci.yml`、`bitbucket-pipelines.yml`、`Jenkinsfile`、`circle.yml` 任何一种 CI 文件。
- `vite.config.ts` 第 13 行 server proxy 硬编码 `http://localhost:18080`，没有 dev-time 环境变量（如 `.env.development`）— 跨开发成员跑不通。
- `vite.config.ts` 没有 `build.outDir`、`build.sourcemap`、`build.chunkSizeWarningLimit`、`build.rollupOptions` 分包策略——更没有 `manualChunks` / `splitVendorChunkPlugin`。
- `tsconfig.json` 把 `noUnusedLocals` 与 `noUnusedParameters` 都设为 `false`，相当于关闭了 dead-code 检查。
- `index.html` 没有 `%env.VITE_*%` 注入或静态 SPA fallback rewrite 配置（即 SPA + nginx/apache 部署需要单独的 try_files）。
- `dist/assets/index-*.css` 约 24 KB，没拆 critical CSS。

### ❌ 缺失
| 严重度 | 缺口 |
|---|---|
| **P0** | 无 lint（ESLint）；`tsc` 也无法防住 React Hooks 滥用 / 空指针 / 不可达分支。 |
| **P0** | 无 format（Prettier）；提交风格、import 排序全靠 PR review 维持。 |
| **P0** | 无 CI pipeline：任何分支都能直接合并到 main。 |
| **P1** | 没有跨环境配置文件（`.env.example` / `.env.development` / `vite-env.d.ts`）：后端地址、WS 协议推导靠硬编码。 |
| **P1** | 没有 `vite build` 后的产物指纹策略（无 `import.meta.env.VITE_*` 暴露给后端的 release 版本号）。 |
| **P1** | 没有 source map 上传 / error reporting（Sentry / Datadog Browser RUM 等）。 |
| **P2** | 没有 `commitlint` / `conventional-changelog` / `release-please`。 |
| **P2** | 没有 docker 镜像（nginx + dist）。 |

---

## 9. 文档（Documentation）

### ✅ 已具备
- 无。`README.md`、`CONTRIBUTING.md`、`STYLEGUIDE.md`、`docs/` 全都没有（仓库根目录与 frontend 根目录都验证过）。
- 项目内仅有的"文档"是源码里的注释，例如：
  - `src/lib/ws.ts` 第 1–6 行解释共享 WS 的动机；
  - `src/lib/api.ts` 第 129–135 行解释 `serviceDetail` 防御；
  - `src/hooks/useHashState.ts` 顶部详细 JSDoc。

### ⚠️ 半成品 / 临时做法
- `components/anim/index.ts` 第 1 行注释提到"inspired by react-bits (DavidHDev/react-bits)"，但代码本身与 react-bits 的具体对应关系没有留链接或 API 差异说明。
- `App.tsx`、`Logs.tsx`、`Metrics.tsx` 多处内联注释夹叙夹议，对一个没有外部文档的库来说，是**主要文档载体**，但显然不够系统。
- 没有 `CHANGELOG`、`LICENSE`、`SECURITY.md`、`CODE_OF_CONDUCT.md`。

### ❌ 缺失
| 严重度 | 缺口 |
|---|---|
| **P0** | 没有 README：用户/二次开发者打开 repo 看不到运行方法、端口列表、依赖图、环境变量、可观测性 demo 步骤。 |
| **P0** | 没有 CONTRIBUTING：新增页面、新增动画、新增 endpoint 都无规范。 |
| **P1** | 没有组件文档库 / Storybook / MDX 站：13 个动画组件的 `<Props>` 表只在源码 JSDoc 看到，IDE 以外拿不到。 |
| **P1** | 没有架构图（前端 ↔ 后端 ↔ WS / 流、代理、监控）。 |
| **P2** | 没有 ADR（架构决策记录）、没有 RFC 模板。 |
| **P2** | 没有"如何新增面板类型"教学文本（`Dashboards.tsx` 的 `PanelRenderer` 第 226 行开始的判级链外人看不懂）。 |

---

## 10. 可视化完整性（Visualization Completeness）

### ✅ 已具备
- **图表库依赖为零**：自己实现的 `TimeSeriesChart`（多 series、x/y 网格、可选图例）、`TraceWaterfall`（按时长比例 + 按 parent 缩进的甘特）、`Sparkline`（侧栏 16×36 微图）、`ServiceMapGraph`（SVG 圆形布局 + 箭头 + 直径按 QPS 缩放 + 颜色按 error 率缩放）。
- 多页都把 `+ seed` 按钮接到了 `api.seed`，demo 友好。
- 时间窗选择器 `TimeRangePicker`，直方图有 `linear` / `log` 切换，20/30/50 buckets。
- `SeverityBadge` 接收 7 个非标准 severity 名称（warning/err/panic 等）做归一化（`src/components/SeverityBadge.tsx` 第 14–24 行），适应 OTLP/老式日志。
- `Histogram`（metrics 页）有 bucket 计数 + p50/p95/p99。
- `ServiceMapPage.tsx` 自写 SVG graph + 边表，提供节点 hover、单击跳转。

### ⚠️ 半成品 / 临时做法

#### 日志查看器（Logs）
- ❌ **没有语法高亮**：日志 `body` 没有任何形态的高亮（无 Prism / Shiki / highlight.js 之类的依赖）。
- ❌ **没有结构化折叠**：attributes 在每行下方用空格串起来（`Logs.tsx` 第 595 行），不是 JSON viewer。
- ❌ **搜索只是子串匹配**：`parseSearchQuery` 不会解释 `AND/OR/NOT/正则`；并且 `parsed.body` 用空格 join 多个裸 token——这意味着多词搜索会被当成 `AND` 的 hidden 行为，但 UI 没有说明。
- ⚠️ **"过滤"只覆盖severity + labels + service，没有时间字段、trace ID 之外的过滤**。
- ❌ **没有虚拟滚动**：硬截断 300 行。
- ❌ **没有时间线密度视图**（如 Grafana 的 histogram 概览条）：整个时间窗只在 `TimeRangePicker` 里给"5m/15m/1h"几个粗粒度选择，没有时间分布柱。
- ⚠️ **没有"暂停后查看历史"的语义隔离**，仅整个 `live` 布尔开关。

#### 链路追踪器（Traces）
- ✅ Trace 列表（每条 trace 卡片）。
- ✅ Trace 详情有 span 表（带 parent 缩进、status、start offset）。
- ✅ 有 `DurationHistogram` 子组件（`Traces.tsx`，但代码段在截断里；可见它存在）。
- ⚠️ **没有火焰图（Flame Graph）**：只有 `TraceWaterfall` 这种横向甘特图；按调用栈深度聚合的 classic flame graph 缺失。
- ⚠️ **没有 span 详情抽屉**：选中 span 只能看到 raw attributes 头三个（`Traces.tsx` 代码末尾 `slice(0,3)`），无法展开 JSON。
- ⚠️ **没有 propagation/上下文链路**：没有 `trace_id → log 自动定位`、`trace_id → metrics 自动定位`；要从 trace 跳到 logs 必须切页面手动输入 ID（`Logs.tsx` 第 605–614 行实现的就是 trace→logs 的入口，但反向不行）。
- ⚠️ **没有 timeline 上 span 的事件（logs attached to span）** 与 span attributes 折叠展开。
- ❌ **没有多 trace 对比（"对比 2 个 trace 的火焰图"）**。
- ❌ **没有 min/max span 折叠折叠 / span 搜索过滤**（只能全局搜 service）。

#### 指标 / Service map / Overview
- ❌ **无 heatmap**：只有 line chart；Grafana 风格 heatmap（按时间×p99 桶）缺失。
- ❌ **无 stacked area / 多 metric 同图堆叠**：`Metrics.tsx` 只能"compare 多 service 同 metric"，不能同 service 多 metric 叠加。
- ❌ **无 gauge / single stat trend spark 之外的 stat 类型**。
- ⚠️ `ServiceMapGraph` 节点用纯圆形布局，没有按依赖层级、cluster、拓扑排序；多达十几个服务时**容易重叠**（`ServiceMapPage.tsx` 第 80 行 `(2 * Math.PI * i) / Math.max(1, nodes.length)` 等分一圈）。
- ⚠️ 没有图例（节点 hover tooltip 上有信息但没有显式颜色 legend）。

### ❌ 缺失（汇总）

| 严重度 | 缺口 |
|---|---|
| **P0** | 日志查看器**无语法高亮、无结构化 JSON 折叠、无字段过滤、无时间直方图、无虚拟滚动**——目前形同 `tail -f` + 简陋表格，不可被视为生产就绪。 |
| **P0** | Trace 查看器**无火焰图、无 span 详情 drawer、无结构化 attributes JSON view**。 |
| **P1** | Trace 缺乏跨页 propagation：从 `Logs` 跳到 `Traces` 只有单向（`Logs.tsx` 第 612 行）；从 Trace 跳到对应日志没有反向；Trace 跳到对应 metric 也无。 |
| **P1** | 缺少 heatmap、gauge、stacked-area、bar chart；目前只有 line chart + 自定义 div-bar 直方图。 |
| **P1** | Service map 没有真正的 graph layout（`d3-force` / `dagre` 等库也未引入），节点一多就散乱。 |
| **P2** | 缺失"对比模式"同时选两个 trace / 两个 service。 |
| **P2** | 缺失 trace timeline 全屏放大模式（Grafana 经典 "Drill into trace"）。 |
| **P2** | 缺失 save-view / dashboard export / annotations 标注。 |

---

## 11. 额外的"非可观测性必要"依赖与冗余动画

> 题目要求"指出被引入但不必要于可观测性仪表盘的动画库"。

### ✅ 自写动画轻量（被广泛使用）
- 13 个动画原语都在 `src/components/anim/`：`BarLoader / BlurText / CountUp / FadeIn / Glitch / GradientText / LiveBadge / Magnetic / Pulse / ShinyText / Stagger / TrendArrow / TrueFocus`。
- 全部**零依赖**——每个都是单文件 React 组件 + 内联 `<style>` 注入 keyframe。

### ⚠️ 但并非每个都对可观测性 dashboard 是必要的

| 动画组件 | 必要性评估 |
|---|---|
| `BlurText`（标题字符模糊入场） | **P2 装饰**：仅在 `Overview.tsx` heading、TopBar 标题、ServiceDetailPage 等约 5 处使用，**对信息密度毫无贡献**，去重眼反而分散注意力。 |
| `Glitch`（RGB-split 抖动） | **P2 装饰**：仅在 avg error 率 > 5% 时使用——当已经用红色徽章 + 大字号表达"严重"时，**叠加抖动属于双通道**，在生产 NOC 屏幕长时间盯着会引起疲劳。 |
| `GradientText`（渐变流动字） | **P2 装饰**：ServiceDetailPage 的服务名用它，glow 动画对数据本身无意义。 |
| `ShinyText`（金属光泽扫过） | 实际**未被任何处引用**（仅在 `anim/index.ts` export 中），半成品。 |
| `TrueFocus`（紫色光晕呼吸） | 实际**未被引用**（仅 export），纯装饰存量。 |
| `Magnetic`（磁吸跟随鼠标） | **P2 装饰**：ServiceDetailPage 用了 5 个包裹 KPI 卡，每次 mousemove 都触发 setState；可观测性应该是"任何状态下都能静态被截图审计"的看板，磁吸反而影响复现性。 |
| `Stagger`（行级瀑布出现） | **P2**：Logs/Live/Overview 行用它做进场；只在 300 行以内有意义，超过即会 cap 24 行（`anim/Stagger.tsx` 第 19 行 `max=24`），是一种半成品。 |
| `CountUp / TrendArrow / Pulse / LiveBadge / BarLoader / FadeIn` | ✅ 都是信息通道——数字滚动、涨跌方向、状态点，确实提升了 observability dashboard 的可读性，应保留。 |

### ❌ 缺（汇总）

| 严重度 | 缺口 |
|---|---|
| **P2** | `ShinyText`、`TrueFocus` 两个组件未被使用——是死代码（export 但无 consumer），应删除或补足用例。 |
| **P2** | 装饰类动画（`BlurText`、`Glitch`、`GradientText`、`Magnetic`）的使用场景明显是"做产品 demo"驱动，而不是"服务 SRE"驱动，可在生产构建里通过 `import.meta.env.PROD` 关闭，省下不必要的 reflow / repaint。 |

---

## 评估总结（一张图）

| # | 维度 | P0 数 | P1 数 | P2 数 | 总体判断 |
|---|------|------|------|------|---------|
| 1 | 认证 & 用户管理 | 3 | 2 | 1 | **Blocker**：完全无登录 |
| 2 | 状态管理 | 2 | 2 | 2 | Blocker：无 Query 层 / AbortController |
| 3 | 性能 | 2 | 3 | 3 | 严重：无 code-split、无虚拟化 |
| 4 | a11y | 2 | 4 | 1 | 严重：键盘可达性、reduced-motion 不完整 |
| 5 | i18n | 1 | 1 | 1 | **Blocker**：整体缺失 |
| 6 | 响应式 | 1 | 2 | 2 | 中：mobile 不可用 |
| 7 | 测试 | 3 | 1 | 1 | **Blocker**：零测试 |
| 8 | CI/CD & 构建 | 2 | 3 | 2 | **Blocker**：无 lint / format / CI |
| 9 | 文档 | 2 | 2 | 2 | **Blocker**：无 README/CONTRIBUTING |
| 10 | 可视化 | 2 | 3 | 3 | 严重：日志/链路核心功能缺 |
| – | 冗余动画 | – | – | 4 | 中：`ShinyText/TrueFocus` 死代码 + 4 个装饰性动画 |

**结论**：这是技术扎实（TypeScript 严格、`shared WebSocket`、`ErrorBoundary`、自写图表）、工程薄弱的 demo 前端。**P0 缺口合计 20 项**，没有任何一项可以让它直接上生产。优先修复顺序建议：

1. 身份认证链路 + 拦截器（`src/lib/api.ts` + 登录页 + token 刷新）。
2. 引入轻量数据层（TanStack Query），同时加 `AbortController` 与 staleness 控制。
3. 路由级 lazy + `Suspense` + 列表虚拟化（`Logs / Traces / Sidebar`）。
4. 加入 ESLint + Prettier + Vitest + Playwright + GitHub Actions 工作流。
5. a11y：`prefers-reduced-motion` 全量审计、键盘焦点环、ARIA landmarks、表格 `scope`、颜色双重通道。
6. i18n：抽离文案 + `react-i18next`。
7. 日志/链路可视化：`react-window` 虚拟化、JSON 代码高亮、火焰图、span 抽屉。
8. 装饰性动画在 prod 关闭或移除（`BlurText`、`Magnetic`、`Glitch`、`GradientText`、`ShinyText`、`TrueFocus`）。
