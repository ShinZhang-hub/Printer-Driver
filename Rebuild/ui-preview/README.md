# ui-preview — UI 隔离测试页面

> **目的**：将 `shared-ui` + `app` 的 UI 部分**单独复制一份**到此目录，方便随意修改、实时预览，确认后再同步回原项目。原项目文件不受影响。

## 结构

```
ui-preview/
  index.html          # 测试页面（左侧控制台 + 右侧 480px 预览卡片）
  vite.config.js      # 端口 1422
  package.json
  src/
    printer-ui.js     # 复制自 Rebuild/shared-ui/printer-ui.js（可随便改）
    shared-style.css  # 复制自 Rebuild/shared-ui/style.css
    app-style.css     # 复制自 Rebuild/app/src/style.css（窗口壳样式）
    mock.js           # 模拟 printer-core 的 InitialState / i18n / runInstall
    main.js           # 预览逻辑：挂载 UI + 控制面板联动
```

## 运行

```bash
cd Rebuild/ui-preview
npm install
npm run dev   # 打开 http://localhost:1422
```

无需 Tauri/Rust，纯浏览器即可预览。`mock.js` 内置了 `config.json` 的 3 个位置、5 语言文案、与 `flow.rs` 一致的 `conflict`/`existing` 计算。

## 控制台能力

- 切换语言（zh/zh-Hant/en/ja/ko）、检测位置（Osaka/Tokyo/空）、本机已有打印机场景（有冲突/无冲突/空/多台长名称）、simple 模式、模拟失败
- 一键预览结果页：成功/失败/跳过（调用 `ui.showResult` 的分组渲染）
- 底部黑底 JSON 实时显示 `request`/`response`/`state`，控制台打印 `[mock] runInstall`

## 修改流程

1. 在 `ui-preview/src/` 内改 `printer-ui.js` / `*.css`，浏览器热更新实时看效果
2. 确认后手动同步回原项目：
   ```bash
   cp Rebuild/ui-preview/src/printer-ui.js Rebuild/shared-ui/printer-ui.js
   cp Rebuild/ui-preview/src/printer-ui.js Rebuild/examples/onboarding-minimal/src/shared-ui/printer-ui.js
   cp Rebuild/ui-preview/src/shared-style.css Rebuild/shared-ui/style.css
   cp Rebuild/ui-preview/src/app-style.css Rebuild/app/src/style.css
   # 或按需只同步某一个
   bash Rebuild/sync-shared-ui.sh  # 若改了 onboarding 内的真源
   ```

## 与原项目隔离

- `ui-preview/src/printer-ui.js` 是**副本**，改这里不会改 `Rebuild/shared-ui/` 或 `Rebuild/app/src/main.js`
- `mock.js` 完全在前端模拟 `getState/getStrings/runInstall`，不依赖 `printer-core` 或 `lpstat`
