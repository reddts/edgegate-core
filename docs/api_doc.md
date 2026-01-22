# API 文档（中文简版）

本项目的完整 API/消息结构由 `docs/edgegate-core-rpc.md` 自动生成（基于 proto），可直接阅读详细字段含义。本文件提供中文概览与使用要点。

## 服务概览（gRPC）
- **Core 服务**（`v2/hcore`）：核心生命周期与状态管理，常用方法：
  - `Start` / `Stop`：启动、停止核心实例。
  - `CoreInfoListener` / `OutboundsInfo` / `MainOutboundsInfo` / `LogListener`：订阅核心状态、出站信息和日志。
  - `SelectOutbound` / `UrlTest`：切换/测试出站。
- **Config 服务**（`v2/config`）：配置解析与构建。
  - 消息 `CoreOptions`、`RouteOptions`、`WarpOptions` 等定义了全局/路由/Warp 配置。
- **Profile/Hello 等服务**：示例/辅助接口，用于演示或基础数据交换。

## 扩展（Extension）
- proto 位于 `extension/extension.proto`、`extension_service.proto`。
- 核心向扩展暴露的能力：UI 更新、弹窗/消息、提交数据、连接前处理等。
- 扩展回调/数据结构均在上述 proto 中定义，WebUI 的 gRPC-Web 代码生成于 `extension/html/rpc.js`。

## 数据与类型
- 所有消息/枚举的字段及含义请查阅 `docs/edgegate-core-rpc.md` 对应章节。
- 配置相关的主要消息：
  - `CoreOptions`：核心全局配置。
  - `RouteOptions` / `Rule`：路由与规则。
  - `WarpOptions` / `WarpWireguardConfig`：Warp 相关配置。

## 调用提示
- gRPC：参考 `docs/edgegate-core-rpc.md` 中的服务与消息定义；客户端可用任意 gRPC 语言绑定。
- Web/浏览器：使用 gRPC-Web，可复用仓库中的生成代码（`extension/html/rpc/*.js`）。
- CLI：`EdgegateCli` 对部分接口做了封装（如 `run`、`parse`、`extension` 调试等），可直接调用 CLI 完成常见操作。

> 如需字段级完整说明，请阅读 `docs/edgegate-core-rpc.md`；本文件仅作为中文索引与导航。***
