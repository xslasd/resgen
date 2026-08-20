# Resgen 示例 (Examples)

这个目录包含了一系列展示 `resgen` 工具各种功能和特性的示例项目。

## 目录结构及使用场景说明

| 目录/文件 | 说明 | 使用场景 |
|---|---|---|
| [`schema/`](./schema) | 包含了所有基础功能的 `.res` 模式 (Schema) 定义文件。 | 学习如何使用 `.res` 语法定义数据结构、路由、状态码、文件上传、自定义类型、装饰器等。也是 `resolver/` 生成代码的输入。 |
| [`resolver/`](./resolver) | 根据 `schema/` 中的文件生成的 Go 语言代码。 | 在应用中直接引用生成的 DTO 模型和接口定义。展示了最常见的代码生成产物。 |
| [`gin/`](./gin) | 一个完整的、基于 Gin 框架运行的示例项目。 | 学习如何将 `resgen` 生成的代码集成到实际的 Gin Web 服务中，包括如何注册路由、实现接口以及启动服务。 |
| [`http/`](./http) | 一个基于 Go 原生 `net/http` 标准库运行的示例项目。 | 学习如何将 `resgen` 生成的代码集成到原生 HTTP 服务中，展示了框架无关的适配能力。 |
| [`resgen.yaml`](./resgen.yaml) | 示例项目的核心代码生成配置文件。 | 在 `examples/` 目录下执行 `resgen generate` 时会读取此配置来生成 `resolver/` 代码。 |

## 如何使用这些示例

1. **学习基础语法**: 浏览 `schema/` 目录下的各个 `.res` 文件，了解 resgen 的 DSL (领域特定语言) 语法。
2. **查看生成结果**: 配合 `schema/` 中的文件，查看 `resolver/` 中生成的 Go 代码，了解 DSL 是如何映射到 Go 结构体和接口的。
3. **运行完整服务**:
   - Gin 版本: 进入 `gin/` 目录，执行 `go run main.go` 启动示例服务 (监听 8080)。
   - 标准 HTTP 版本: 进入 `http/` 目录，执行 `go run main.go` 启动示例服务 (监听 8081)。

## 高级特性演示

- **文件上传/下载**: 参考 `schema/06_file.res`。
- **自定义标量**: 参考 `schema/03_scalar.res` 和 `resolver/scalars.go`。
- **装饰器和验证器**: 参考 `schema/02_decorator_validator.res`。
- **状态码包装**: 参考 `schema/05_status_wrap.res`。
- **Content-Type 别名**: 参考 `schema/04_content_type.res`。
- **联合类型**: 参考 `schema/07_union.res`。
