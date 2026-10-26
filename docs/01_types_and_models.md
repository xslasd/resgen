# 数据类型系统与建模规范 (Types & Models)

数据类型系统是 Resgen DSL 的核心基石。Resgen 采用声明式、强类型的类型系统，确保从契约编写、编译生成到运行时校验的全链路类型安全。

---

## 1. 内置基础类型 (Scalar Types)

Resgen 提供了标准化的基础标量类型，自动映射到 Go 语言的原生类型与验证逻辑：

| DSL 类型 | Go 类型 | 传输层表现 | 语义说明与应用边界 |
| :--- | :--- | :--- | :--- |
| `String` | `string` / `*string` | 字符串 | 文本字符序列。 |
| `Int` | `int` / `*int` | 整数 | 32/64 位自适应整数。在 JSON 传输中映射为标准数字。 |
| `Float` | `float64` / `*float64` | 浮点数 | 双精度浮点数值。 |
| `Boolean` | `bool` / `*bool` | 布尔值 | `true` 或 `false`。 |
| `File` | `*multipart.FileHeader` / `*LocalFileDownload` | 文件二进制流 | 入参映射为文件上传头；出参统一映射为 `*LocalFileDownload` 强类型文件下载载体。详见 [文件流与上传下载实战](./04_file_and_stream.md)。 |
| `Any` | `any` | 任意 JSON/对象 | 动态结构的“逃生舱”，映射为 Go 的 `any`，绕过编译期强类型约束。 |
| `Field` | `any` | 字段引用元数据 | **专用于校验器参数声明**的元类型，用于跨字段对比（如密码一致性校验）。⚠️ **绝对不能作为普通的模型属性类型**。 |

---

## 2. 类型修饰符与指针映射规则

Resgen 严格继承了 GraphQL 风格的修饰符设计，同时针对 Go 语言生态进行了深度的工程化语义对齐：

- **必填约束 (`!`)**：在标量或切片后追加 `!` 表示该字段必填（Non-null）。
  - **未标注 `!` 的字段**：在 Go 侧统一映射为**指针类型**（如 `*string`、`*int`、`*[]string`）。这样可以精准表达前端“未传此字段（`nil`，保留原值）”与“传了零值/空串（更新为空）”的业务语义差异；
  - **标注了 `!` 的字段**：在 Go 侧映射为**值类型**（如 `string`、`int`、`[]string`），代码生成器会自动注入非空检查与校验拦截。
- **数组声明 (`[]`)**：使用方括号包裹类型声明数组切片。

---

## 3. 数组修饰符组合与校验全景对比

当数组 `[]` 与感叹号 `!` 组合使用时，感叹号的位置决定了约束是作用在切片本身还是切片内的元素上：

| DSL 语法 | 生成的 Go 结构体类型 | 切片本身约束 | 数组元素约束 | 校验逻辑与传参表现 | 推荐程度 |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `String` | `*string` | 可选（可为 null） | - | 传空或未传跳过校验 | 常用（可选标量） |
| `String!` | `string` | **必填（非空）** | - | 未传或传空字符串拦截并报错 | 常用（必填标量） |
| `[String]` | `*[]*string` | 可选（可为 null） | 可选（可为 null） | 允许切片为 null，且允许切片内存在 null 元素占位 | 极少使用 |
| `[String!]` | `*[]string` | **可选（可为 null 或 `[]`）** | **严格非空（元素不可为空）** | 切片本身不报错；仅当切片内含有空项（如 `[""]`）时拦截 | 🌟 **极力推荐（最常用的可选列表）** |
| `[String]!` | `[]*string` | **必须传递切片** | 可选（可包含 null） | 传 null 或不传直接拦截；允许切片内元素为 null | 极少使用 |
| `[String!]!` | `[]string` | **必须传递非空切片** | **严格非空** | 切片不可为 null 且长度需大于 0，内部元素也不可为空 | 🌟 **极力推荐（最常用的必填列表）** |

> [!TIP]
> **工程最佳实践铁律：“数组内部永远加感叹号，绝不在数组里留指针”**
> 在绝大多数业务场景下，列表内包含 `nil` 指针毫无意义，反而会导致前端序列化出 `[null, "a"]`，业务端每次遍历还需判断 `if item != nil`。
> - 若列表本身可选：优先使用 **`[Type!]`**（映射为 `*[]Type`，空列表可直接通过）；
> - 若列表必须传且不能为空：优先使用 **`[Type!]!`**（映射为 `[]Type`，非空切片）。

---

## 4. `Any` 与 `Field` 的深度对比

### 4.1 `Any`：动态结构“逃生舱”
当接入第三方未定义结构的 Webhook、或是完全动态的 JSON 配置时，使用 `Any` 可以跳过强类型检查：

```graphql
type DynamicPayload {
    event: String!
    data:  Any!        # Go 中映射为 any，支持任意 JSON 对象/数组/基础类型
}
```

### 4.2 `Field`：跨字段引用校验元类型
`Field` **只能**在 `validator` 的形参列表中使用，它代表对模型中另一个字段名称的引用：

```graphql
# 声明一个时间先后校验器：要求当前字段的值必须早于 targetField 字段的值
validator timeBefore(targetField: Field)

input DateRangeInput {
    start_time: String! @timeBefore(end_time)
    end_time:   String!
}
```

在生成的校验逻辑中，框架会自动从当前结构体中提取 `end_time` 的值，并将其传给校验器实现的参数，无需手动通过反射反射字段。

---

## 5. 核心模型定义 (Models)

Resgen 将模型区分为 **实体模型 (`type`)** 和 **输入模型 (`input`)**：

### 5.1 实体模型 (`type`) 与输入模型 (`input`)

```graphql
# 业务实体模型（通常用于接口出参、数据库映射）
type User {
    id:        Int!
    username:  String!
    nickname:  String
    avatar:    String
    created_at: Int!
}

# 输入模型（专门用于接收请求参数）
input CreateUserInput {
    username: String!
    password: String!
    nickname: String
}
```

### 5.2 泛型模型定义
支持通过尖括号 `<T>` 声明泛型模型，大幅减少重复模型的编写：

```graphql
type PageResult<T> {
    total: Int!
    page:  Int!
    size:  Int!
    items: [T!]!
}

# 使用泛型模型：
# PageResult<User> -> 生成强类型分页结构
```

---

## 6. 枚举类型 (Enums)

Resgen 支持强类型枚举定义。生成的 Go 代码包含常量声明、合法性自校验方法（`IsValid()`）以及请求参数自反序列化方法（`FromParam`）。

### 6.1 字符串枚举 (默认)

```graphql
# 默认底层为字符串枚举
enum UserRole {
    Admin = "admin"
    Editor = "editor"
    Viewer = "viewer"
}
```

生成的 Go 代码形态：
```go
type UserRole string

const (
    UserRole_Admin  UserRole = "admin"
    UserRole_Editor UserRole = "editor"
    UserRole_Viewer UserRole = "viewer"
)

func (e UserRole) IsValid() bool {
    switch e {
    case UserRole_Admin, UserRole_Editor, UserRole_Viewer:
        return true
    }
    return false
}
```

### 6.2 整数枚举

```graphql
# 显式指定底层类型为 Int
enum Status: Int {
    Draft = 0
    Published = 1
    Archived = 2
}
```

---

## 7. 联合多态类型 (Unions)

当接口字段需要根据某个“类型判别器”（Discriminator）返回截然不同的数据结构时，可以使用联合类型：

```graphql
# 1. 定义判别器枚举
enum ContentKind {
    Article = "article"
    Video = "video"
}

# 2. 定义具体模型
type ArticleContent {
    title:   String!
    content: String!
}

type VideoContent {
    duration: Int!
    play_url: String!
}

# 3. 声明联合类型（以 ContentKind 枚举为判别器）
union PostPayload(ContentKind) {
    Article: ArticleContent
    Video:   VideoContent
}

# 4. 在模型中引用联合类型
type PostItem {
    id:      Int!
    type:    ContentKind!
    payload: PostPayload! @discriminator(type)
}
```

### 7.1 编译期与运行时保障
- 编译器会严格校验 `union` 的判别器必须指向有效的 `enum` 模型；
- 生成的代码内置了 `decodeUnionPayload` 自适应反序列化函数，支持从 `json.RawMessage`、`map[string]any`、原始 `string` 或结构体等多种介质精准平铺转换为目标强类型指针。

---

## 8. 响应包装器 (Wrappers)

使用 `wrap` 关键字定义标准化响应信封外壳，统一接口成功与失败时的 JSON 结构：

```graphql
# 1. 单对象响应包装器
wrap ResData<T> {
    code: Int!
    msg:  String!
    data: T
}

# 2. 分页列表响应包装器
wrap ListRes<T> {
    rows:  [T!]!
    total: Int!
}

# 3. 树形结构原生包装器
wrap TreeRes<T> {
    items: [T!]!
    total: Int!
}
```

关于响应包装器的全局默认配置、接口局部覆盖、彻底关闭（`wrap=none`）以及嵌套单态化展开，请详细参阅 [统一响应控制与错误处理](./08_response_handling.md)。
