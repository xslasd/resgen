# 切面装饰器与校验系统 (Decorators & Validators)

在大型 Web 服务开发中，横切关注点（如认证鉴权、限流审计、字段校验、数据脱敏等）如果不加约束，往往会散落在各个 Controller 或业务逻辑中。

Resgen 引入了声明式的 **切面装饰器 (Decorator)** 与 **上下文感知校验器 (Validator)**，帮助团队构建高内聚、低耦合的企业级架构。

---

## 1. 装饰器系统 (Decorators)

使用 `decorator` 关键字声明可被挂载到接口上的切面策略。通过 `stage`（执行阶段）和 `scope`（作用域）两个维度进行正交组合：

```graphql
decorator Name(Arg1: Type, ...) [stage=..., scope=...]
```

### 1.1 元属性详解

| 属性键 | 可选值 | 说明与典型应用场景 |
| :--- | :--- | :--- |
| **`stage`** | `request` (默认) | **请求前置阶段**：在参数提取与模型绑定前执行。适合全局身份认证（Token 校验）、IP 黑名单、全局频控限流。 |
| | `invoke` | **业务执行前阶段**：在参数绑定与验证通过后、调用业务 Handler 前执行。适合基于具体参数的权限细查（如检查操作者是否为该数据的所有者）。 |
| | `response` | **响应后置阶段**：在业务 Handler 返回后、渲染响应前执行。适合结果脱敏（如手机号中间 4 位掩码）、出参统一加工、业务审计日志。 |
| **`scope`** | `global` (默认) | **全局作用域**：在全局 `Decorator` 接口中统一声明，所有模块共用同一套底层实现。 |
| | `specialized` | **特化作用域**：在具体模块的 `Resolver` 接口中特化声明，由该业务模块的 Handler 独享实现，支持个性化拦截。 |

### 1.2 完整组合示例

```graphql
# 1. 全局请求拦截：JWT 鉴权（所有模块共用）
decorator auth(role: String) [stage=request, scope=global]

# 2. 特化业务前置：校验当前用户是否为文章所有者（仅在 Article 模块中实现）
decorator checkOwner [stage=invoke, scope=specialized]

# 3. 特化响应脱敏：对特定返回对象进行数据脱敏
decorator maskEmail [stage=response, scope=specialized]

group /articles {
    # 组合应用多个装饰器
    @auth("admin")
    @checkOwner
    @maskEmail
    POST /:id => UpdateArticle(id: Int! @path, content: String!): Article
}
```

### 1.3 生成的 Go 接口形态

#### 全局装饰器接口 (`Decorator[T any]`)
```go
type Decorator[T any] interface {
    Auth(ctx T, info MethodInfo, role string) error
}
```

#### 特化装饰器方法（直接生成在 `ArticleResolver` 接口中）
```go
type ArticleResolver[T any] interface {
    // 业务方法
    UpdateArticle(ctx context.Context, id int, content string) (*Article, error)

    // 特化 invoke 装饰器拦截
    OnInvoke_checkOwner_UpdateArticle(ctx T, info MethodInfo, input *UpdateArticleArgs) error

    // 特化 response 装饰器拦截（支持对 result 进行修改）
    OnResponse_maskEmail_UpdateArticle(ctx T, info MethodInfo, input *UpdateArticleArgs, result *Article, err error) (*Article, error)
}
```

---

## 2. 权限标识映射与文档挂锁高亮

对于中后台系统，前端常常需要知道每一个接口需要什么权限码，以便动态控制页面按钮的显示与隐藏。

Resgen 提供了**零耦合权限映射机制**：

### 2.1 全局配置约定 (`resgen.yaml`)
```yaml
generator:
  # 指定作为权限标识的装饰器名称（忽略大小写，@ 前缀可省略）
  auth_decorator: "auth"

  # 指定装饰器中代表权限码的参数名称
  auth_param_name: "role"
```

### 2.2 自动生效的工程红利
配置生效后，代码生成器会全自动完成以下工作：
1. **API 契约提取**：在导出的 `docs/api.json` 中，自动注入标准的 `"permission": "admin"` 属性，前端脚手架可直接读取该属性生成权限路由表；
2. **交互式文档金锁高亮**：在 `docs/api.html` 预览文档中，接口标题旁会自动渲染精致的金黄色 `🔒 admin` Badge；左侧导航栏对应的接口也会渲染挂锁图标，便于接口审计与前后端协同。

---

## 3. 字段校验器 (Validators)

Resgen 支持在字段级别声明业务校验规则。通过 `validator` 关键字扩展自定义校验器：

```graphql
# 声明一个邮箱格式校验器
validator email

# 声明带参数的长度范围校验器
validator range(min: Int, max: Int)

input RegisterInput {
    email:    String! @email
    password: String! @range(min=8, max=32)
}
```

### 3.1 上下文感知校验契约 (`Validator[T any]`)
生成的 `Validator` 接口天然具备请求上下文感知能力（传入 `ctx T`）：

```go
type Validator[T any] interface {
    Email(ctx T, fieldName string, value any) error
    Range(ctx T, fieldName string, value any, min int, max int) error

    // EnumError 处理枚举校验失败的统一多语言与结构化映射
    EnumError(ctx T, fieldName string, enumType string, value any) error
}
```

> [!TIP]
> 因为入参包含了请求上下文 `ctx`（例如 `*gin.Context`），业务校验实现可以方便地读取客户端语言头（`Accept-Language`）或多租户上下文，从而返回精准本地化的错误消息。

---

## 4. 业务逻辑接管 (Takeover Directives)

当某些接口的参数结构极其复杂，或者需要跳过框架自动生成的解析/校验逻辑时，可以使用逻辑接管指令：

### 4.1 `@customBind`（接管参数绑定）
声明 `@customBind` 后，生成器将**不再生成私有的 `bindXXX` 逻辑**，而是在模块的 `Resolver` 接口中暴露绑定契约：

```graphql
POST /legacy/webhook @customBind => ProcessWebhook(input: WebhookData!): Result
```

生成的业务接口需要实现自定义绑定：
```go
// 业务需要自行实现如何从底层 Request 中反序列化 input
BindProcessWebhook(request ServerContextBase, input *WebhookData) error
```

### 4.2 `@customValidate`（接管参数校验）
当校验逻辑依赖复杂的数据库连表查询、或是跨系统的远程 RPC 校验时，声明 `@customValidate`：

```graphql
POST /orders @customValidate => SubmitOrder(input: OrderInput!): Result
```

生成的业务接口将暴露：
```go
// 校验通过返回 nil，不通过返回 error
ValidateSubmitOrder(ctx T, input *OrderInput) error
```
