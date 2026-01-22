贡献指南（中文）

环境要求
- 需要安装正确版本的 Go（当前模块使用 go 1.24）。检查版本：`go version`。

开发代码
- Go 代码位于 `edgegate-core`，桌面入口在 `edgegate-core/custom`，移动端入口在 `edgegate-core/mobile`。
- 桌面端需将 Go 代码编译为 C 共享库。提供的 `Makefile` 支持多平台：
  - `make windows-amd64`
  - `make linux-amd64`
  - `make macos-universal`
- 移动端使用 `gomobile` 工具：
  - `make android`
  - `make ios`
- 构建产物会输出到 `edgegate-core/bin`。

若只使用已有产物可忽略上述步骤；如需修改或调试，请先确保 Go 工具链、Make 及相关依赖（gomobile、NDK、Xcode 等）已准备好。***
