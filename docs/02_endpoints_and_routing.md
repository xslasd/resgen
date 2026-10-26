# 接口编排与参数提取机制 (Endpoints & Routing)

Resgen 提供了一套高度直观、紧凑且表达力极强的接口声明语法。通过模块化分组、智能参数提取与统一的非 GET 参数合成机制，帮助开发者彻底告别繁琐的路由样板代码。

---

## 1. 模块与路由分组 (Module & Group)

### 1.1 模块声明 (`module`)
每个 `.res` 文件必须以 `module` 关键字开头，声明当前文件所归属的业务模块。同一模块名称的多个 `.res` 文件会被编译器自动合并：

```graphql
module UserCenter
```

### 1.2 路由分组 (`group`)
使用 `group` 可以对 URL 路径进行分组，并批量应用组级别的元属性与前缀：

```graphql
group /users [prefix="/api/v1", wrap=ResData] {
    # 最终完整路由为: GET /api/v1/users/:id
    GET /:id => GetUser(id: Int! @path): User

    # 最终完整路由为: POST /api/v1/users
    POST / => CreateUser(input: CreateUserInput!): User
}
```

> [!NOTE]
> - `prefix` 元属性会与 group 的基本路径自动拼接；
> - 组级声明的元属性（如 `wrap=ResData`）会被组内所有接口自动继承，除非单个接口显式覆盖。

---

## 2. 接口定义基本语法 (Endpoints)

Resgen 的接口定义遵循经典且直观的映射箭头符号 `=>`：

```graphql
HTTP_METHOD /route/path [MetaOptions] => HandlerName(Arguments): ReturnType [MetaOptions]
```

- **HTTP 动词自动归一化**：支持 `GET`、`POST`、`PUT`、`DELETE`、`PATCH`。若在 DSL 中书写小写（如 `get`、`post`），编译器会在词法分析阶段自动归一化为标准大写。
- **路径参数语法**：使用冒号 `:` 声明动态路径占位符（如 `/:id`、`/orgs/:org_id/users/:user_id`）。

---

## 3. 四维参数提取来源体系 (Parameter Sources)

Resgen 支持从 HTTP 请求的四个物理来源中精准提取参数：

| 来源指令 | 对应 HTTP 物理层 | 提取规则与典型场景 |
| :--- | :--- | :--- |
| **`@query`** | URL Query 参数 (`?page=1&size=10`) | 筛选、分页、搜索关键字。 |
| **`@path`** | URL Path 动态占位符 (`/users/:id`) | 资源唯一标识主键。参数名称必须与路径占位符 `:name` 严格一致。 |
| **`@header`** | HTTP Request Header | 客户端设备号、网关凭证、密钥版本号（如 `device_id: String! @header`）。 |
| *(无标注)* | HTTP Request Body | 请求体反序列化对象（JSON、Form、XML 等）。 |

```graphql
group /api {
    # 综合示例：跨来源参数混合提取
    GET /orders/:order_id => QueryOrder(
        order_id:  String! @path,    # 来自路径占位符 :order_id
        verbose:   Boolean @query,   # 来自 Query ?verbose=true
        client_ip: String  @header   # 来自请求头 X-Client-Ip (自动转下划线/中划线适配)
    ): OrderDetail
}
```

### 3.1 参数重命名别名 (`@alias`)
当前端请求的物理键名与 Go 结构体命名规范冲突、或存在连字符（如 `X-Device-Id`）时，使用 `@alias` 指令进行平滑映射：

```graphql
input FilterInput {
    pageSize:   Int!    @alias("page_size")     # 将 Query 中的 page_size 映射到 Go 的 PageSize
    appKey:     String! @alias("x-app-key")     # 映射带连字符的 Header 或 Body 键
}
```

---

## 4. 非 GET 请求平铺参数自动合成机制 (Consistency Rule)

在传统 API 开发中，非 GET 请求（`POST`、`PUT`、`PATCH`、`DELETE`）的参数绑定常常存在歧义：到底该读 Query 还是读 Body？

Resgen 制定了行业领先的**一致性合成铁律**：

### 4.1 核心规则
1. **GET 请求**：未显式标注来源的平铺参数，**默认全部绑定至 URL Query**；且禁止在 GET 接口中传递嵌套结构体，强制接口扁平化；
2. **非 GET 请求 (POST/PUT/PATCH/DELETE)**：
   - 凡是没有显式标注 `@query`、`@path`、`@header` 的平铺形参，**统一全部从 HTTP 请求体 (Body) 中反序列化**；
   - 编译器会自动在底层合成一个强类型的 `XxxArgs` 输入结构体（例如方法名为 `UpdateUser`，则自动生成 `UpdateUserArgs` 结构体），并将这些参数聚合；
   - OpenAPI 3.0.3 规范文档会自动将这些参数标记在 `requestBody` 契约中，保持前后端完全一致。

### 4.2 语法对比示例

#### 场景 A：显式传递单个 Input 结构体
```graphql
POST /users => CreateUser(input: CreateUserInput!): User
# 业务 Handler 签名: CreateUser(ctx context.Context, input *CreateUserInput) (*User, error)
```

#### 场景 B：平铺多参数（由编译器全自动合成包装）
```graphql
POST /users/bind => BindCard(
    device_id: String! @header,  # 从 Header 提取
    card_no:   String!,          # 自动归入 Body
    cvv:       String!           # 自动归入 Body
): Result
```

生成的 Go 业务接口签名会自动保持整洁，并将 Body 参数封装为参数对象：
```go
// 编译器自动合成的包装模型
type BindCardArgs struct {
    CardNo string `json:"card_no"`
    Cvv    string `json:"cvv"`
}

// Resolver 接口签名
BindCard(ctx context.Context, device_id string, input *BindCardArgs) (*Result, error)
```

> [!TIP]
> 如果在非 GET 请求中，某些参数确实需要从 URL Query 提取（例如 `POST /upload?category=doc`），只需显式追加 `@query` 即可：
> ```graphql
> POST /upload => Upload(category: String! @query, file: File!): Result
> ```

---

## 5. 接口参数文档说明与注释智能提取

Resgen 提供了**双模文档注释解析器**，能够从 DSL 源码中全自动提取参数释义并渲染到 `api.html` 交互式文档中。

### 5.1 方案 A：多行展开局部注释（适合复杂接口）
当接口参数较多时，可以在括号内换行并附带单行注释：

```graphql
# 复杂搜索分页接口
POST /articles/search => SearchArticles(
    # 搜索关键字（匹配标题与内容）
    keyword: String,

    # 当前页码（从 1 开始）
    page: Int!,

    # 每页展示条数（最大允许 100）
    pageSize: Int! @alias("page_size")
): PageResult<Article>
```

### 5.2 方案 B：单行极简接口行内智能提取（🚀 业界独创）
对于参数紧凑的接口，无需换行展开，直接在接口定义上方通过 `# 参数名 详细说明` 的格式编写，Resgen 编译器内置的智能解析引擎会自动提取对应关系：

```graphql
# 返回单个文章详情
# id   欲查询的文章唯一自增 ID 主键
# lang 国际化语言包代码 (如 zh-CN, en-US)
GET /articles/:id => GetArticle(id: Int! @path, lang: String @query): Article
```

在生成的 `api.json` 和 `api.html` 中，`id` 和 `lang` 参数的 Description 会被精准提取并完美渲染，兼顾极简排版与高质量文档交付！
