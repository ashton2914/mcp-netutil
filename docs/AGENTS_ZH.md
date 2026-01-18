# mcp-netutil 代理指南

本仓库包含了一个基于 Go 的模型上下文协议 (MCP) 服务器实现，提供网络实用工具（路由跟踪 traceroute、Ping/延迟检测、系统状态统计）。

## 1. 构建、Lint 检查和测试命令

使用标准的 Go 工具链。本项目支持通过 `build.sh` 进行交叉编译。

### 构建
- **本地开发构建**:
  ```bash
  go build .
  ```
- **跨平台发布构建**:
  ```bash
  ./build.sh
  ```
  (查看 `dist/` 目录获取构建产物)

### 测试
- **运行所有测试**:
  ```bash
  go test ./...
  ```
- **运行特定包的测试**:
  ```bash
  go test -v ./pkg/latency
  ```
- **运行单个测试函数**:
  ```bash
  # 语法: go test -v [package_path] -run [TestName]
  go test -v ./pkg/latency -run TestParsePingOutput
  ```
- **运行带有竞态检测的测试**:
  ```bash
  go test -race ./...
  ```

### Lint / 验证
- **标准 Go Vet**:
  ```bash
  go vet ./...
  ```
- **格式检查**:
  ```bash
  # 确保代码已格式化
  go fmt ./...
  ```

## 2. 代码风格与规范

遵循地道的 Go 惯用规范 (Effective Go)。

### 格式化与布局
- **工具**: 提交前务必运行 `go fmt`。
- **缩进**: 使用 Tab（标准 Go 行为）。
- **行长**: 保持行长合理，但如果强制 80 字符换行影响可读性，则不必严格遵守。
- **分组**: 将导入包分为标准库、第三方库和本地包。

### 命名规范
- **大小写**: 导出的标识符使用 `PascalCase`（大驼峰），内部标识符使用 `camelCase`（小驼峰）。
- **首字母缩写**: 保持缩写词大小写一致（例如 `serveHTTP`、`JSONRPCRequest`、`sessionID`、`ID`）。
- **文件名**: 使用 `snake_case`（下划线命名，例如 `latency_test.go`）。
- **测试函数**: `TestXxx`（例如 `TestParsePingOutput`）。

### 类型与结构体
- **JSON 标签**: API 结构体（MCP 消息、RPC）必须包含 `json` 标签。
  ```go
  type Tool struct {
      Name        string `json:"name"`
      Description string `json:"description"`
  }
  ```
- **组合**: 适当使用结构体组合。

### 错误处理
- **包装**: 使用 `fmt.Errorf("context: %w", err)` 包装错误。
- **检查**: 显式处理错误。`if err != nil { return ... }`。
- **返回值**: 优先使用 `(ResultType, error)` 模式。

### 并发与上下文 (Context)
- **上下文**: 对于涉及 I/O 或长时间运行的操作，将 `context.Context` 作为第一个参数传递（例如 `latency.Run(ctx, ...)`）。
- **取消**: 在循环或耗时操作中响应 `ctx.Done()`。
- **同步**: 对共享状态使用 `sync.RWMutex`（如 `main.go` 中的 `sessions` 映射）。
- **通道**: 使用通道 (Channel) 在 SSE 处理程序和后台工作程序之间进行通信。

### 测试
- **模式**: 对具有多种输入/输出排列的逻辑使用 **表格驱动测试 (Table-Driven Tests)**。
  ```go
  func TestSomething(t *testing.T) {
      tests := []struct {
          name     string
          input    string
          expected string
          wantErr  bool
      }{
          {"Case 1", "in", "out", false},
      }
      for _, tt := range tests {
          t.Run(tt.name, func(t *testing.T) { ... })
      }
  }
  ```
- **断言**: 使用标准的 `if got != want { t.Errorf(...) }`。目前不使用第三方断言库（保持简单）。

### 包结构
- **根目录**: `main.go` 包含服务器入口点和 HTTP/MCP 连线逻辑。
- **pkg/**: 领域逻辑位于此处。
  - `pkg/latency`: Ping 包装器和解析逻辑。
  - `pkg/traceroute`: 路由跟踪执行。
  - `pkg/system`: 系统统计信息。
  - `pkg/cache`: SQLite 数据库交互。

### 依赖
- **管理**: 使用 `go modules`。
- **CGO**: 构建时默认禁用 (`CGO_ENABLED=0`) 以确保静态链接，除非涉及 `modernc.org/sqlite`（这是一个无 CGO 的端口，兼容 `CGO_ENABLED=0`）。

### 日志
- **输出**: 日志输出到 `stderr` (`log.SetOutput(os.Stderr)`)。
- **格式**: 标准 `log` 包（默认包含时间戳）。

---
*由 opencode 生成*
