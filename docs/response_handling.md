# Resgen 响应处理机制

Resgen 通过强类型的包装器（Wrappers）和响应适配器（Responder）提供统一且可预测的响应结构。

## 1. 响应包装器 (Wrappers)

包装器是使用 `wrap` 关键字定义的模型，用于标准化返回格式。

```graphql
# 定义一个通用的响应包装器
wrap ResData<T> {
    code: Int     # 业务状态码
    msg: String    # 提示信息
    data: T        # 业务数据
}
```

## 2. 响应流转契约 (The Contract)

在生成的代码中，所有的响应路径都遵循以下流程：

1.  **逻辑执行**：业务方法返回 `(result, err)`。
2.  **状态码转换**：通过 `Responder.ErrorToStatus(ctx, err)` 确定 HTTP 状态码。
3.  **包装绑定**：通过 `Responder.Bind{Wrapper}(ctx, result, err)` 将数据填充进包装模型。
4.  **最终渲染**：调用 `ServerContext.Render{Format}` 输出结果。

**生成的 Go 代码示例**：
```go
if err != nil {
    // 错误处理：data 传 nil，err 传实际错误
    request.RenderJson(e.r.ErrorToStatus(ctx, err), e.r.BindResData(ctx, nil, err))
    return
}
// 成功处理：使用元数据定义的 successStatus (默认为 200)
request.RenderJson(200, e.r.BindResData(ctx, result, nil))
```

## 3. 响应接管与自定义 (Takeover & Customization)

如果默认的包装逻辑无法满足需求，可以通过以下方式进行接管：

### A. 实现特化响应装饰器 (Specialized Response Decorator)
当你需要对特定接口的结果进行加工（如脱敏、动态修改状态码）时：
```graphql
decorator maskEmail [stage=response, scope=specialized]

group /users {
    @maskEmail
    GET /profile => GetProfile(): User
}
```
在实现 `OnResponse_maskEmail_GetProfile` 时，你可以修改返回的 `result`。


## 4. 响应包装器覆盖机制 (`wrap` 与 `wrap=none`)

为了在简化接口定义的同时保持绝对的排版灵活性，Resgen 提供了一套**组级默认 + 接口级按需覆盖**的响应包装器机制。

### A. 组级默认包装器
在一个路由组（`group`）上，你可以定义一个通用的成功/错误外壳，使全组内的接口默认自动享受统一包装逻辑：
```graphql
group /products [wrap=ResData] {
    # 默认套壳：成功返回 200 + ResData<Product>
    GET /:id => GetProduct(id: Int @path): Product
}
```

### B. 接口级局部覆盖
如果你在组级配置了统一包装，但在该组中的某个接口需要使用特殊的包装器，可以在接口元数据中直接覆盖它：
```graphql
group /products [wrap=ResData] {
    # 覆盖组级默认：此接口将使用 PageResult 包装，生成 PageResult<[Product]>
    GET /list => ListProducts(): [Product] [wrap=PageResult]
}
```

### C. 彻底关闭包装器 (`wrap=none`)
在很多场景下，我们**不希望有任何响应外壳包裹**。此时可以在接口元数据中填入 `wrap=none` 彻底关闭包装逻辑：
```graphql
DELETE /:id => DeleteProduct(id: Int @path): String [state=204, wrap=none]
```

#### `wrap=none` 的三大经典黄金场景：
1. **防止双重套壳**：当接口声明的返回类型本身就是自定义包装器（如 `PageData<T>`）时，避免外层再被包裹成 `ResData<PageData<T>>`。
2. **纯原始/流式数据输出**：如返回纯文本（`ctype=text`）、字节流或文件下载（`File`），不应受到 JSON 外壳干扰。
3. **空内容响应 (204 No Content)**：当指定成功状态码为 `state=204` 时，配合 `wrap=none` 可以使得接口成功后**直接返回空响应体**，完美顺应 HTTP/RESTful 规范。

---

## 5. 生成的代码流对比 (Go Code Generation Differences)

以下是启用响应包装与使用 `wrap=none` 时，代码生成器生成的处理函数（Executor Handler）的核心代码逻辑差异对比：

### 🧬 情况 A：启用包装器时 (默认或覆盖包装)
代码生成器会通过 `Responder.Bind{Wrapper}` 将您的业务返回值与错误统一塞进响应外壳，然后进行渲染：
```go
result, err := e.biz.GetProduct(request.Context(), idVal)
if err != nil {
    // 渲染带有包装外壳的 JSON 错误响应
    request.RenderJson(e.r.ErrorToStatus(request.Native(), err), e.r.BindResData(request.Native(), nil, err))
    return
}
// 渲染带有包装外壳的 JSON 成功响应
request.RenderJson(200, e.r.BindResData(request.Native(), result, nil))
```

### 🧬 情况 B：声明 `wrap=none` 时
代码生成器将**直接渲染原始的业务返回值**，若接口配置了 `etype` (如错误时仍用统一格式包装)，错误分支仍会被外壳安全保护：
```go
result, err := e.biz.DeleteProduct(request.Context(), idVal)
if err != nil {
    // 若配置了 etype=json，这里会降级为 ResData 包装；若未配置，则直接渲染错误字符串
    request.RenderJson(e.r.ErrorToStatus(request.Native(), err), err.Error())
    return
}
// 完美降维：直接渲染原始值（204 状态下直接返回空）
request.RenderJson(204, result)
```

---

## 6. 状态码与元数据控制

通过 DSL 中的元数据 `[...]` 可以精准控制接口响应行为：

- **`state`**: 指定接口成功执行时的 HTTP 状态码（如 200, 201, 202, 204 等）。
- **`ctype`**: 指定成功响应的 Content-Type 类型（映射到 context 不同的渲染函数，如 `json`, `text`, `xml`, `multipart` 等）。
- **`etype`**: 指定错误响应的包装格式。这实现了“成功时直接返回纯原始文件流，失败时退化为标准 JSON 统一结构报错”的弹性设计。

## 5. API 文档展示

Resgen 的交互式文档会自动解析响应结构：
- **泛型还原**：文档会显示 `ResData<User>` 而非模糊的 JSON。
- **字段展开**：展示包装器及其内部业务模型的完整字段列表。
- **状态码标注**：基于 `[state=...]` 元数据清晰标注预期成功状态码。
