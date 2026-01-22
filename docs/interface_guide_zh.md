# 接口与调用说明（edgegate-core）

## 产物与入口
- CLI：`EdgegateCli`（二进制在 `bin/`，源码入口 `cmd/bydll`）。
- 桌面/服务器共享库：`edgegate-core.dll`（Windows）、`edgegate-core.so`（Linux）、`edgegate-core.dylib`（macOS），入口在 `platform/desktop`。
- 移动端库：`edgegate-core.aar`（Android）、`EdgegateCore.xcframework`（iOS），入口在 `platform/mobile`。
- WebUI：由 `assets/webui-src` 构建后复制到 `bin/webui`。

## CLI 常用命令（简要）
- 启动核心：`EdgegateCli run --config <URL或文件路径> [-d <edgegate.json>]`
- 解析/检查配置：`EdgegateCli parse --config <URL或文件路径>`
- 生成证书：`EdgegateCli gen-cert ...`
- 扩展调试：`EdgegateCli extension`（本地调试扩展服务）
- 其他子命令可查看：`EdgegateCli --help`

## 配置与数据
- 主配置可通过 `--config` 指定；若提供 `edgegate.json`（或自定义 JSON），通过 `-d` 传入。
- 默认配置结构定义见 `v2/config`，序列化与解析逻辑在 `v2/config/parser.go` 等文件。

## 扩展（Extension）接口
- 入口与注册：`extension` 目录；gRPC 定义见 `extension/*.proto`。
- UI 与交互：`extension.UpdateUI`、`extension.ShowDialog`、`extension.ShowMessage`。
- 数据与流程：`extension.SubmitData`、`extension.BeforeAppConnect`、`e.Base.Data` 持久化。
- 独立实例/解析：`extension/sdk.RunInstance`、`extension/sdk.ParseConfig`。
- 扩展在所有平台显示，由核心调用扩展提供的回调实现。

## gRPC/Proto 位置
- 核心接口 Proto：`v2/hcore/*.proto`（生成文件同目录）。
- 配置 Proto：`v2/config/*.proto`。
- 扩展接口 Proto：`extension/*.proto`。
- JS gRPC-Web 代码（WebUI 使用）：`extension/html/rpc` 及生成的 `extension/html/rpc.js`。

## WebUI 构建与调用
- 源码：`assets/webui-src`（React/Vite）。
- 构建：`make webui`（会在 `assets/webui-src` 执行 `npm install` + `npm run build`，产物复制到 `bin/webui`）。
- 核心 CLI 在启动时会将 WebUI 作为静态资源提供（需按平台打包的二进制/库）。

## 构建与集成注意
- Go 版本 1.24，CGO 启用；多平台编译参数见 `Makefile`。
- 本地依赖替换：`go.mod` 中 `replace` 指向 `localmods/*`，确保离线可用。
- CI 需安装 Go、Node（用于 WebUI）、以及平台工具链（NDK、Xcode、MinGW 等，视目标而定）。
