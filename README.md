# edgegate-core

Edgegate Core 基于 sing-box 的多协议代理内核，提供 CLI、桌面/移动端共享库和扩展 SDK，可运行于 Windows、macOS、Linux、Android、iOS。通过扩展机制，可为应用添加自定义 UI、对话框、配置前置处理等能力。

## 快速使用（Docker）
- 拉取镜像：`docker pull ghcr.io/reddts/edgegate-core:latest`
- Compose 示例：
  ```sh
  git clone https://github.com/reddts/edgegate-core
  cd edgegate-core/platform/docker
  docker-compose up
  ```
  通过环境变量 `CONFIG` 指定远端配置；如需本地配置，可将 `edgegate.json`（或自定义文件）挂载到容器 `/degegate/edgegate.json`。

## 扩展能力
- 自定义 UI/对话框/消息：`extension.UpdateUI`、`extension.ShowDialog`、`extension.ShowMessage`
- 配置与数据：`extension.SubmitData`、`extension.BeforeAppConnect`、`e.Base.Data` 持久化
- 运行独立实例与解析：`extension/sdk.RunInstance`、`extension/sdk.ParseConfig`

## 构建概览
- Go 版本：1.24（见 `go.mod`）
- 多平台目标：参考 `Makefile`（`windows-amd64`、`linux-amd64`、`linux-arm64`、`macos`、`android`、`ios` 等）。
- Windows 快捷脚本：`build_windows.bat`（构建 Windows x64 DLL 与 CLI）。
- Docker 镜像构建：`platform/docker/Dockerfile`。
- WebUI 本地构建：`assets/webui-src`（npm 构建，产物复制到 `bin/webui`）。

## 文档
- 接口与调用说明：`docs/interface_guide_zh.md`
- 贡献指南：`CONTRIBUTING.md`
- 许可证：`LICENSE.md`
