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


## 4. 状态码与元数据控制

通过 DSL 中的元数据 `[...]` 可以精准控制响应行为：

- **`state`**: 指定成功时的 HTTP 状态码。
- **`ctype`**: 指定成功响应的 Content-Type（对应不同的渲染函数）。
- **`wrap`**: 覆盖当前接口使用的包装器。

**示例**：
```graphql
# 导出接口：成功返回 201，且不使用默认包装器，直接返回文本
GET /export => Export(): String [state=201, ctype=text, wrap=none]
```

## 5. API 文档展示

Resgen 的交互式文档会自动解析响应结构：
- **泛型还原**：文档会显示 `ResData<User>` 而非模糊的 JSON。
- **字段展开**：展示包装器及其内部业务模型的完整字段列表。
- **状态码标注**：基于 `[state=...]` 元数据清晰标注预期成功状态码。
