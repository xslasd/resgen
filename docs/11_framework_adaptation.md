# Web 框架适配实战指南 (Framework Adaptation)

Resgen 采用了彻底的**框架无关设计 (Framework-Agnostic)**。生成的核心代码（DTO、接口契约、Executor 执行流）不强依赖任何第三方 Web 框架（如 Gin、Echo、Fiber、Chi 或原生 `net/http`）。

开发者只需提供一个实现了 `ServerContextBase` 契约的适配器，即可让生成的 API 代码在任意 Go Web 框架上高性能运行。

---

## 1. 框架适配层契约 (`ServerContextBase`)

生成的 `engine.gen.go` 中定义了标准适配接口：

```go
type ServerContextBase interface {
    // 1. 基础参数获取 (用于标量及基础类型)
    GetPath(name string) string
    GetQuery(name string) string
    GetHeader(name string) string

    // 2. 请求体绑定 (JSON / Form / Multipart / XML 等)
    Payload(source BodySource, dest any) error

    // 3. 底层响应控制
    RenderRaw(code int, contentType string, body []byte)
    RenderStream(code int, localFileDownload LocalFileDownload)
    Context() context.Context

    // 4. 类型化格式渲染方法 (根据 DSL 中出现的 ctype 动态生成)
    RenderJson(code int, obj any)
    RenderText(code int, obj any)
    RenderXml(code int, obj any)
}
```

---

## 2. Gin 框架生产级适配实战

以下是基于 Gin 框架的完整适配器实现（参考 `examples/gin/main.go`）：

```go
package main

import (
    "context"
    "fmt"
    "mime/multipart"
    "net/http"

    "github.com/gin-gonic/gin"
    "your-project/resolver"
)

// GinContext 封装 *gin.Context 并实现 resolver.ServerContextBase
type GinContext struct {
    GC *gin.Context
}

// 确保在编译期严格实现 ServerContext 契约
var _ resolver.ServerContext[*gin.Context] = (*GinContext)(nil)

func (c *GinContext) Native() *gin.Context {
    return c.GC
}

func (c *GinContext) Context() context.Context {
    return c.GC.Request.Context()
}

func (c *GinContext) GetPath(name string) string {
    return c.GC.Param(name)
}

func (c *GinContext) GetQuery(name string) string {
    return c.GC.Query(name)
}

func (c *GinContext) GetHeader(name string) string {
    return c.GC.GetHeader(name)
}

func (c *GinContext) Payload(source resolver.BodySource, dest any) error {
    switch source {
    case resolver.SourceJson:
        return c.GC.ShouldBindJSON(dest)
    case resolver.SourceForm:
        return c.GC.ShouldBind(dest)
    case resolver.SourceMultipart:
        return c.GC.ShouldBind(dest)
    default:
        return fmt.Errorf("unsupported body source: %s", source)
    }
}

func (c *GinContext) RenderJson(code int, obj any) {
    c.GC.JSON(code, obj)
}

func (c *GinContext) RenderText(code int, obj any) {
    c.GC.String(code, "%v", obj)
}

func (c *GinContext) RenderXml(code int, obj any) {
    c.GC.XML(code, obj)
}

func (c *GinContext) RenderRaw(code int, contentType string, body []byte) {
    c.GC.Data(code, contentType, body)
}

func (c *GinContext) RenderStream(code int, dl resolver.LocalFileDownload) {
    if dl.ContentType != "" {
        c.GC.Header("Content-Type", dl.ContentType)
    }
    c.GC.FileAttachment(dl.FilePath, dl.Filename)
}
```

---

## 3. 原生 `net/http` 标准库适配实战

针对极致轻量化、微服务或 Serverless 场景，可以使用 Go 原生 `net/http` 适配器（参考 `examples/http/main.go`）：

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"
    "path/filepath"

    "your-project/resolver"
)

type HttpContext struct {
    W http.ResponseWriter
    R *http.Request
}

func (c *HttpContext) Native() *http.Request {
    return c.R
}

func (c *HttpContext) Context() context.Context {
    return c.R.Context()
}

func (c *HttpContext) GetPath(name string) string {
    return c.R.PathValue(name) // Go 1.22+ 原生路由占位符
}

func (c *HttpContext) GetQuery(name string) string {
    return c.R.URL.Query().Get(name)
}

func (c *HttpContext) GetHeader(name string) string {
    return c.R.Header.Get(name)
}

func (c *HttpContext) Payload(source resolver.BodySource, dest any) error {
    body, err := io.ReadAll(c.R.Body)
    if err != nil {
        return err
    }
    return json.Unmarshal(body, dest)
}

func (c *HttpContext) RenderJson(code int, obj any) {
    c.W.Header().Set("Content-Type", "application/json; charset=utf-8")
    c.W.WriteHeader(code)
    _ = json.NewEncoder(c.W).Encode(obj)
}

func (c *HttpContext) RenderText(code int, obj any) {
    c.W.Header().Set("Content-Type", "text/plain; charset=utf-8")
    c.W.WriteHeader(code)
    _, _ = fmt.Fprint(c.W, obj)
}

func (c *HttpContext) RenderRaw(code int, contentType string, body []byte) {
    c.W.Header().Set("Content-Type", contentType)
    c.W.WriteHeader(code)
    _, _ = c.W.Write(body)
}

func (c *HttpContext) RenderStream(code int, dl resolver.LocalFileDownload) {
    f, err := os.Open(dl.FilePath)
    if err != nil {
        c.W.WriteHeader(http.StatusInternalServerError)
        return
    }
    defer f.Close()

    c.W.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", dl.Filename))
    if dl.ContentType != "" {
        c.W.Header().Set("Content-Type", dl.ContentType)
    }
    c.W.WriteHeader(code)
    _, _ = io.Copy(c.W, f)
}
```

---

## 4. 路由挂载钩子与一键行号秒级跳转 (`GetHandlerJumpPath`)

在启动服务时，通过 `engine.BindRegister` 挂载路由注册钩子。

Resgen 生成器内置了利用**反射与运行时 PC 指针定位**的高级工具函数 `GetHandlerJumpPath`。在控制台打印路由表时，支持直接输出**相对工作区路径与定义行号**：

```go
func main() {
    r := gin.Default()
    engine := resolver.NewEngine[*GinContext]()

    // 挂载路由钩子
    engine.BindRegister(func(e *resolver.Engine[*GinContext], info resolver.MethodInfo, handler resolver.HandlerFunc[*GinContext]) {
        // 在终端打印包含行号的直达链接
        fmt.Printf("[ROUTE] %-6s %-25s -> %s\n", info.Method, info.Path, info.HandlerPos)

        r.Handle(info.Method, info.Path, func(c *gin.Context) {
            handler(&GinContext{GC: c}, info)
        })
    })

    // 注册业务 Resolver 模块
    userExecutor := resolver.NewUserCenterExecutor(engine, &UserHandler{})
    userExecutor.Mount()

    _ = r.Run(":8080")
}
```

控制台输出效果：
```text
[ROUTE] GET    /api/v1/users/:id         -> handler/user.go:42 (GetUser)
[ROUTE] POST   /api/v1/users             -> handler/user.go:88 (CreateUser)
```
在 VSCode 或 GoLand 终端中，直接按住 `Ctrl + 点击` 即可**一秒直达具体业务函数的物理源文件行号**，极大地提升团队调试效率！
