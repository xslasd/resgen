# Resgen DSL 语法指南

Resgen 采用声明式、强类型的 DSL (Domain Specific Language) 来定义 API 契约。

## 0. 数据类型系统 (Type System)

Resgen DSL 拥有一套强类型的系统，确保从接口定义到代码生成的类型安全性。

### 内置基础类型
| DSL 类型 | Go 类型 | 说明 |
| :--- | :--- | :--- |
| `String` | `string` | 字符串 |
| `Int` | `int` | 整数 |
| `Float` | `float64` | 浮点数 |
| `Boolean` | `bool` | 布尔值 |
| `Time` | `time.Time` | 时间/日期 |
| `File` | `*multipart.FileHeader` | 文件上传/下载流 |
| `Any` | `any` | 逃生舱，表示任意动态数据类型 |
| `Field` | `any` | 字段引用类型（专用于校验器参数声明，⚠️ 警告：绝对不能作为普通属性字段类型！） |

### 类型修饰符
- **必填校验 (`!`)**：在类型后追加 `!` 表示该字段不能为空。Resgen 会根据此修饰符自动生成 `nil` 检查或 `required` 校验逻辑。
  - 示例：`String!` (必填字符串), `User!` (必填对象)。
- **数组声明 (`[]`)**：使用方括号包裹类型表示数组/切片。
  - 示例：`[Int]` (可选整数数组), `[User!]!` (必填的非空用户对象数组)。

### 自定义类型与泛型
- **模型引用**：直接使用定义过的 `type` 或 `input` 名称。
- **泛型支持**：通过 `<T>` 语法在 `wrap` 或 `type` 中使用泛型，提高结构复用性。
  - 示例：`ResData<[User!]!>`。

### 💡 深度探讨：`Any` 与 `Field` 的区别与高阶用法

很多开发者会混淆 `Any` 与 `Field`。在 Resgen 的强类型设计中，二者承担着截然不同的核心职责：

#### 1. `Any` 类型：未定义/动态结构的“逃生舱”
`Any` 代表**数据的动态承载**。它在生成 Go 代码时，对应物理类型是 `any`（空接口 `interface{}`）。
- **典型场景**：当您需要承载一个不确定的 JSON Map 散列，或者返回一个完全动态的结构时。
  ```graphql
  input CreateTaskInput {
      title: String!
      metadata: Any  # 前端可以传任意形式的附加 JSON 数据
  }
  ```
- > [!CAUTION]
  > **Go 侧的 Scalar 避坑死锁规避**：
  > 如果您在 DSL 中定义了标量 `scalar A: Any`，在 Go 侧实现该标量时，**绝对不要**声明为 `type A any`。因为 Go 语言不允许在 interface 类型上挂载方法，会导致标量契约方法 `ToValue/FromValue` 无法定义。
  > **正确做法**：在 Go 侧将 `A` 声明为实体类型，例如 `type A map[string]any` 或 `type A struct { Value any }`，这 100% 能够顺利通过编译！

#### 2. `Field` 类型：高级【字段引用】类型
`Field` 并不用于承载普通的 API 数据，它**专门且仅能在校验器 (Validator) 的参数声明中出现**！
> [!WARNING]
> **绝对使用禁忌**：
> `Field` 是一种“专用的元数据类型”，它**绝对不能**在普通的输入/输出模型（如 `type`、`input`、`wrap`）中作为属性字段的类型声明！如果您需要声明一个承载任意或动态散列数据的属性，请 **100% 选用 `Any` 类型**！
- **定位**：它告诉 Resgen 编译器——“这是一个高阶的字段绑定参数，在生成代码时，请进行智能探测并将参数字符串升格为强类型的 Go 物理变量路径！”
- **典型场景（跨字段关联校验）**：
  在 DSL 中定义跨字段校验器，参数类型使用 `Field!`：
  ```graphql
  # 1. 声明校验器，指定参数为 Field 引用
  validator timeBefore(targetField: Field!)

  input TaskPeriod {
      # 2. 传入 "endTime"，编译器会自动在 TaskPeriod 里查找同名字段
      startTime: IntTime! @timeBefore("endTime")
      endTime: IntTime!
  }
  ```
- **生成结果对比（智能升格 vs 安全兜底）**：
  - **使用 `Field!` 引用已存在字段**：若传入 `"endTime"`（在模型中存在），代码生成器会将其自适应转换为 **Go 变量**：`input.Period.EndTime`，用于跨字段值比对。
  - **安全退化**：若参数类型声明的是普通 `String!`，或传入了模型中不存在的字段（如 `@timeBefore("abc")`），编译器会将其安全退化为普通的**字符串常量**：`"abc"`。这彻底消除了二义性冲突！

##### 🚀 跨层级引用与“最高公共祖先”设计原则
在复杂的 API 参数设计中，属性往往是层层嵌套的。`Field` 类型原生且完美地支持**跨层级的深度穿透校验**：
* **向下跨层级（Dot notation 深度穿透）**：
  只要以当前所处的 Model 为根节点，您可以使用 `.` 点分语法深度向下引用子模型的属性。
  ```graphql
  input CreateTaskInput {
      title: String!
      # 💡 在父级上，通过 "period.startTime" 智能穿透引用子模型的字段进行关联校验
      period: TaskPeriod! @timeBefore("period.startTime", "period.endTime")
  }

  input TaskPeriod {
      startTime: IntTime!
      endTime: IntTime!
  }
  ```
  在代码生成时，Resgen 编译器会像导航仪一样自动顺藤摸瓜检索，将上述跨层级引用精准升格转换为强类型的 Go 物理变量：**`input.Period.StartTime`** 与 **`input.Period.EndTime`**！
  
* **为什么不支持“向上反向引用”？**：
  子模型（如 `TaskPeriod`）是通用的“数据积木”，在定义时是完全解耦孤立的，它可能被多个不同的父级模型引用。如果允许子模型内部去硬编码反向依赖某个特定的父级属性，就会造成致命的**高耦合与循环依赖**，彻底丧失了独立复用能力。

* **💡 黄金最佳实践：最高公共祖先原则 (Lowest Common Ancestor)**：
  如果您遇到涉及多个跨层级字段的复杂关联校验，**最健康的架构做法是：将校验器挂载在能同时“俯瞰”到这两个字段的“最高公共祖先节点（通常是顶层/父级 Model）”上！** 
  这不仅能顺畅地向下进行点分路径穿透，更是从设计上强迫您写出高内聚、低耦合的殿堂级高品质 API 契约！

---

## 1. 核心声明项

### 模型定义 (Models)
用于定义数据结构，支持 `type` (输出) 和 `input` (输入)。

```go
type User {
    id: Int!
    username: String!
    avatar: String
}

input CreateUserInput {
    username: String! @min(5)
    password: String!
}
```

### 响应包装器 (Wrappers)
使用 `wrap` 关键字定义通用的响应格式，支持泛型 `T`。

```go
wrap ResData<T> {
    code: Int
    msg: String
    data: T
}
```

### 模块与组 (Groups)
使用 `group` 组织相关的接口。

```go
@auth("admin")  # 组级装饰器，会自动应用到组内所有接口
group /users [wrap=ResData] {

    @loginRequired  # 接口级装饰器，会追加到组级装饰器之后
    GET /profile => GetProfile(): ResData<User>

    POST /login => Login(username: String, password: String): ResData<Token>
}
```

**组级 Meta** 支持以下键：
| 键     | 说明                              | 示例             |
|--------|-----------------------------------|------------------|
| `wrap` | 针对组内所有接口的默认错误包装器  | `[wrap=ResData]` |
| `state`| 接口成功时的默认 HTTP 状态码      | `[state=200]`    |
| `ctype`| 接口成功时的默认响应 Content-Type | `[ctype=json]`   |

## 2. 接口定义 (Endpoints)

### 方法、路径与 Content-Type 别名
支持标准 HTTP 方法（不区分大小写，生成时自动转为大写）。通过在方法路径后加 **`[ctype=别名]`** 指定请求 Content-Type：

- `POST /path [ctype=json]`：默认 JSON（可省略）。
- `POST /path [ctype=form]`：表单提交。
- `POST /path [ctype=multipart]`：文件上传。

> [!IMPORTANT]
> **别名的解析逻辑**：
> 括号中的关键字通过 `resgen.yaml` 中的 `content_type_aliases` 映射到标准 MIME 类型：
> ```yaml
> content_type_aliases:
>   form: "application/x-www-form-urlencoded"
>   multipart: "multipart/form-data"
> ```
> 如需支持 `xml` 等格式，只需在配置文件中添加映射即可。

```go
# 请求使用 multipart 提交
POST /avatar [ctype=multipart] => UploadAvatar(file: File!): ResData<String>

# GET 请求无需指定 ctype
GET /users/:id => GetUser(id: Int @path): User
```

> [!TIP]
> **响应 Content-Type 别名**：
> 响应元数据中的 `ctype` (成功时) 和 `etype` (失败时) 也是别名，会根据 `resgen.yaml` 配置映射到真实的 MIME 类型。Resgen 会根据这些使用到的类型，在生成的 `engine.gen.go` 中自动生成对应的类型化渲染方法（如 `RenderJson`, `RenderText`）。


### 参数绑定
- `@path`: 路径参数
- `@query`: 查询参数
- `@header`: 请求头
- `@customBind`: **逻辑接管绑定**。跳过自动生成代码的绑定逻辑，在 Resolver 中生成 `BindXXX` 方法供手动实现（适用于 Multipart 复杂处理或自定义解析）。
- 默认逻辑：
  - `GET` 方法：参数默认绑定到 `Query`。
  - `POST/PUT/PATCH` 方法：Body 字段的序列化格式由接口的 `ctype` 统一决定（`json`/`form`/`multipart`），**无需在字段上逐个标注**。

> [!NOTE]
> `@path`/`@query`/`@header` 字段可以出现在 `input` 结构体字段中，也可以直接作为顶层参数。
> `File` 类型字段会自动推断为 multipart 传输，若未指定 `ctype`，生成器会自动将接口的 `ctype` 升级为 `multipart`；
> 若明确指定了 `ctype=json` 或 `ctype=form` 但 input 中含有 `File` 字段，生成器会在生成阶段报错。


### 参数规范
- **GET 扁平化**：针对 `GET` 接口，如果参数是结构体，Resgen 会自动将其字段展开为顶层查询参数（如 `?page=1&size=10`）。
- **禁止嵌套**：`GET` 接口的结构体参数不允许包含嵌套结构体。

### 接口参数文档说明与注释智能提取

为了保证生成的交互式 API 文档中，前端能清晰看到每个请求参数的精确用途与业务规则，Resgen DSL 提供了两种极其强悍的参数文档书写机制。

#### 1. 方案 A：多行展开局部注释（适合多参数、复杂接口）
因为 Resgen 词法解析器原生支持括号内参数的换行，当接口参数较多或需要详细文档时，推荐将参数展开为多行，并在**参数定义的前一行紧贴书写 `#` 注释**：

```graphql
# 复杂分页搜索文章
GET /search => SearchArticles(
    # 检索关键字（可选模糊匹配）
    keyword: String,
    
    # 当前查询的页码数（从 1 开始）
    page: Int,
    
    # 每页返回的最大条数限制（最大不超过 100）
    size: Int
): ResData<[Article]>
```
> [!NOTE]
> 此时，顶部的 `# 复杂分页搜索文章` 会被精准识别为 Endpoint 本身的主文档说明；而各参数前方的注释则会被百分之百绑定到各自对应的参数描述中！

#### 2. 方案 B：单行定义智能提取（适合极简接口 —— 🚀 业界独创智能分析器！）
当您喜欢将简单的接口写在单行以保持 DSL 排版紧凑时，您可以在接口定义上方的注释块中，通过特定前缀行直接为接口入参附加专属说明：

```graphql
# 返回单个对象：ResData<Article>
# id  欲查询的文章唯一自增 ID 主键
GET /:id => GetArticle(id: Int @path): ResData<Article>
```
Resgen 编译器内置的 **`智能注释分析器 (Smart Comment Analyzer)`** 会自适应扫查注释行，一旦识别出形如 `# 参数名 [描述]` 格式的行，它会自动完成两全其美的处理：
1. **自动提取绑定**：将 `# id  欲查询的文章...` 这一行剥离并精准绑定为 `id` 参数的 `Doc` 属性；
2. **纯净化主文档**：自动从 Endpoint 的主说明文案中把这几行参数注释剔除，使主文档保持为干净剔透的 `"返回单个对象：ResData<Article>"`！

##### 💡 匹配规则支持
只要在一行的 `#` 符号后，写上 **`[参数名] [至少两个空格/制表符/冒号/横杠] [描述]`** 即可被分析器完美捕获。以下格式均 100% 支持：
- `# id  ArticleID` （连续双空格）
- `# id: ArticleID` （冒号分割）
- `# id - ArticleID` （横杠分割）

## 3. 装饰器与校验器

- **装饰器 (Decorator)**：用于注入中间件逻辑（如认证）。
- **校验器 (Validator)**：用于字段校验。

```graphql
GET /check => CheckEmail(email: String @email): Any
```

> [!NOTE]
> **校验顺序优先级**：生成的代码会优先执行所有字段的 `Required` 校验，全部通过后再按顺序执行业务语义校验（如 `@min`, `@email` 等）。

### 装饰器元数据 (Meta)
在定义 `decorator` 时，可以通过 `[...]` 配置其执行阶段和作用域：

| 键 | 可选值 | 说明 |
|---|---|---|
| `stage` | `request` (默认), `invoke`, `response` | **执行阶段**：<br> - `request`: 参数绑定前（适合鉴权、限流）<br> - `invoke`: 业务逻辑调用前（适合权限细查、上下文注入）<br> - `response`: 结果返回前（适合结果脱敏、日志审计） |
| `scope` | `global` (默认), `specialized` | **实现作用域**：<br> - `global`: 在全局 `Decorator` 接口中实现，所有模块共用。<br> - `specialized`: 在模块 `Resolver` 中特化实现，由各业务模块独立控制。 |

**示例**：
```go
# 全局请求装饰器：所有模块共用的认证逻辑
decorator auth [stage=request, scope=global]

# 特化执行装饰器：仅由当前 Resolver 实现，校验资源归属
decorator checkOwner [stage=invoke, scope=specialized]

# 特化响应装饰器：仅由当前 Resolver 实现，对特定数据脱敏
decorator maskEmail [stage=response, scope=specialized]
```

```graphql
# 逻辑接管示例 (手动校验)
POST /complex_logic @customValidate => Process(data: ComplexInput): Result
```

## 4. 逻辑接管 (Takeover)
当你需要跳过框架自动生成的绑定或校验逻辑，改由业务层手动控制时，可以使用以下指令：

- **`@customBind`**: 生成器将不再生成 `bindXXX` 私有方法，而是在 `Resolver` 接口中暴露 `BindXXX(request ServerContextBase, input *InputModel) error`。
- **`@customValidate`**: 生成器将不再生成 `validateXXX` 私有方法，而是在 `Resolver` 接口中暴露 `ValidateXXX(ctx T, input *InputModel) error`。

**使用场景**：
- `customBind`: 需要手动解析复杂的 `multipart/form-data`、或者处理非标准协议的 Payload。
- `customValidate`: 校验逻辑依赖外部数据库查询、或者存在复杂的跨字段关联校验逻辑。

## 5. 响应映射
**接口响应元数据**通过返回类型后的方括号 `[...]` 配置：

| 键      | 说明                             | 示例                |
|---------|----------------------------------|---------------------|
| `state` | 成功时的 HTTP 状态码             | `[state=201]`       |
| `ctype` | 成功响应的 Content-Type（别名）  | `[ctype=text]`      |
| `etype` | 错误响应的 Content-Type（别名）  | `[etype=json]`      |
| `wrap`  | 覆盖错误包装器（接口级最高）     | `[wrap=ResData]`    |

> [!TIP]
> `etype` 未指定时，自动 fallback 到 `resgen.yaml` 中 `default_content_type` 对应的 MIME 类型。

```go
# 201 状态码
POST /users => CreateUser(...): User [state=201]

# 响应文本，错误时返回 JSON 并使用 ResData 包装
GET /export => ExportUsers(): String [ctype=text, etype=json, wrap=ResData]

# 组合：请求 ctype + 响应 state
POST /upload [ctype=multipart] => Upload(file: File!): String [state=201, wrap=ResData]
```

> [!NOTE]
> **优先级规则**：接口级 Meta > 组级 Meta > `resgen.yaml` 全局配置。

## 5. 框架适配要求
Resgen 不直接依赖特定 Web 框架。为了让生成的代码正常工作，适配层需要实现生成的 `resolver.Context[T]` 接口。

### 类型化渲染接口
生成的 `engine.gen.go` 会基于 DSL 中出现的 Content-Type 自动扩展如下接口：

```go
type Context[T any] interface {
    engine.ServerContext[T]
    
    RenderJson(code int, obj any)
    RenderText(code int, obj any)
    # ... 其他在 DSL 中定义过的类型
}
```

开发者在实现适配器（如 `GinContext`）时，**必须**提供这些 `Render{Type}` 方法的实现。

## 6. 全局配置 (resgen.yaml)

DSL 的生成行为可以通过项目根目录下的 `resgen.yaml` 进行深度定制。

### 代码生成配置
- **`package`**: 指定生成代码的 Go 包名。
- **`struct_tags`**: 定义结构体标签的生成策略（如 `json`, `form`）及命名风格（`snake`, `camel`, `lower`）。
- **`default_content_type`**: 接口未定义时的默认请求/响应类型别名（如 `json`）。
- **`content_type_aliases`**: 定义别名（如 `form`）到标准 MIME 类型（如 `application/x-www-form-urlencoded`）的映射。
- **`scalar_style`**: 自定义标量的代码生成风格，可选 `isolation`（默认）或 `direct`，详见下方 [自定义标量](#7-自定义标量-scalar) 章节。

### API 文档配置
- **`base_url`**: 文档中显示的 API 基准请求地址。
- **`doc_case`**: 统一 API 文档中呈现的字段命名风格。

### 默认响应行为
- **`default_ok_status`**: 接口未定义 `state` 时默认的成功状态码。
- **`default_wrap`**: 接口未定义 `wrap` 时默认应用到的全局错误包装器。

---

## 7. 自定义标量 (Scalar)

自定义标量允许你将 DSL 中的一个简单基础类型（如 `Int`）在业务层映射为任意有意义的 Go 类型（如 `time.Time`），并完整控制其在 API 传输层的序列化/反序列化行为。

### DSL 语法

```graphql
# 声明一个名为 IntTime 的自定义标量，网络传输层使用 Int (int64)
scalar IntTime: Int
```

然后即可在模型与接口中直接使用：

```graphql
type ScalarOutput {
    createdAt: IntTime
}

group /api [wrap=ResData] {
    GET /items/:time => GetItem(time: IntTime @path): ScalarOutput
}
```

### resgen.yaml 配置

在 `scalars` 节中声明标量对应的 Go 物理实现路径：

```yaml
scalars:
  IntTime:
    model: "github.com/yourname/yourproject/scalars.IntTime"
```

> [!NOTE]
> 生成器会在运行时通过 AST 静态分析自动推导出 `model` 所指向类型的底层 Go 类型，并在生成代码中自动添加对应的 `import` 语句，**无需手动配置 `target`**。

---

### 两种生成风格 (`scalar_style`)

通过 `resgen.yaml` 中的 `scalar_style` 属性，可以在两种截然不同的代码生成策略之间切换：

```yaml
generator:
  scalar_style: "isolation"   # 方案 A：DTO 隔离流（默认）
  # scalar_style: "direct"   # 方案 B：标量直通流
```

#### 方案 A：`isolation`（DTO 隔离流）—— 默认

生成器会在业务层与传输层之间引入独立的 DTO 结构体作为隔离缓冲，通过 `ToDTO/FromDTO` 方法进行显式转换。

**业务对象中使用原生 Go 类型（如 `time.Time`），开发体验最佳：**

```go
// 生成的业务实体（使用原生类型）
type ScalarOutput struct {
    CreatedAt *time.Time `json:"created_at"`
}

// 生成的传输层 DTO（使用网络基础类型）
type ScalarOutputDTO struct {
    CreatedAt *int64 `json:"created_at"`
}

// 生成的转换方法（由框架自动调用，无需手写）
func (m *ScalarOutput) ToDTO(ctx any) (*ScalarOutputDTO, error) { ... }
func (m *ScalarOutput) FromDTO(ctx any, dto *ScalarOutputDTO) error { ... }
```

**Go 侧标量实现须实现的接口：**

| 方法 | 接收者 | 签名 | 用途 |
|:--|:--|:--|:--|
| `FromParam` | `*IntTime` | `func (it *IntTime) FromParam(ctx any, s string) error` | 从路径/查询/Header 的字符串参数中解析 |
| `FromValue` | `*IntTime` | `func (it *IntTime) FromValue(ctx any, v int64) error` | 从请求 Body 的基础类型（如 `int64`）中反序列化 |
| `ToValue` | `IntTime` | `func (it IntTime) ToValue(ctx any) (int64, error)` | 将标量序列化为传输层基础类型（如 `int64`） |

> [!IMPORTANT]
> `FromValue` 的第二个参数类型和 `ToValue` 的第一个返回值类型，**必须**与 DSL 中声明的 BaseType（如 `scalar IntTime: Int` 中的 `Int` → Go 类型 `int64`）完全一致。生成器会在生成期进行 AST 强校验，不匹配时报错并终止生成。

**完整示例（`isolation` 模式）：**

```go
package scalars

import (
    "strconv"
    "time"
)

type IntTime time.Time

// FromParam：处理 Path/Query/Header 参数（string → IntTime）
func (it *IntTime) FromParam(ctx any, s string) error {
    sec, err := strconv.ParseInt(s, 10, 64)
    if err != nil {
        return err
    }
    *it = IntTime(time.Unix(sec, 0))
    return nil
}

// FromValue：处理 Body 反序列化（int64 → IntTime）
func (it *IntTime) FromValue(ctx any, v int64) error {
    *it = IntTime(time.Unix(v, 0))
    return nil
}

// ToValue：序列化到传输层（IntTime → int64）
func (it IntTime) ToValue(ctx any) (int64, error) {
    return time.Time(it).Unix(), nil
}
```

---

#### 方案 B：`direct`（标量直通流）

生成器不生成 DTO 结构体和 `ToDTO/FromDTO` 方法。业务实体中直接使用标量类型本身（如 `scalars.IntTime`），由标量类型自行负责序列化行为。

**生成的代码更简洁，文件体积减少约 60%：**

```go
// 生成的业务实体（直接使用标量类型）
type ScalarOutput struct {
    CreatedAt *scalars.IntTime `json:"created_at"`
}
// ✅ 无 DTO，无转换方法
```

**Go 侧标量实现须实现的接口：**

| 方法 | 接收者 | 签名 | 用途 |
|:--|:--|:--|:--|
| `FromParam` | `*IntTime` | `func (it *IntTime) FromParam(ctx any, s string) error` | 从路径/查询/Header 的字符串参数中解析（两种模式均需要） |

> [!NOTE]
> `direct` 模式下，Body 的序列化/反序列化完全由用户自行决定。框架对具体序列化库（`encoding/json`、`sonic`、`msgpack` 等）和实现方式**零约束**。

**使用标准 `encoding/json` 时的完整示例（`direct` 模式）：**

```go
package scalars

import (
    "encoding/json"
    "strconv"
    "time"
)

type IntTime time.Time

// FromParam：处理 Path/Query/Header 参数（两种模式均需要）
func (it *IntTime) FromParam(ctx any, s string) error {
    sec, err := strconv.ParseInt(s, 10, 64)
    if err != nil {
        return err
    }
    *it = IntTime(time.Unix(sec, 0))
    return nil
}

// MarshalJSON：控制 JSON 序列化（IntTime → int64 时间戳）
func (it IntTime) MarshalJSON() ([]byte, error) {
    return json.Marshal(time.Time(it).Unix())
}

// UnmarshalJSON：控制 JSON 反序列化（int64 时间戳 → IntTime）
func (it *IntTime) UnmarshalJSON(data []byte) error {
    var sec int64
    if err := json.Unmarshal(data, &sec); err != nil {
        return err
    }
    *it = IntTime(time.Unix(sec, 0))
    return nil
}
```

---

### 两种模式对比总结

| 维度 | `isolation`（默认） | `direct` |
|:--|:--|:--|
| **业务层字段类型** | 原生 Go 类型（如 `*time.Time`） | 标量别名（如 `*scalars.IntTime`） |
| **生成代码量** | 较多（含 DTO + 转换方法） | 极简（无 DTO） |
| **是否支持 `ctx` 传递** | ✅ 支持（转换时可获取请求上下文） | ❌ 不支持（序列化无状态） |
| **序列化库约束** | 框架内置，使用 `ToValue/FromValue` | 完全自由，用户自定义 |
| **必须实现的方法** | `FromParam` + `FromValue` + `ToValue` | `FromParam`（+ 自定义序列化逻辑） |
| **适用场景** | 需要 ctx 注入（多租户、时区转换等） | 纯格式转换，追求极简代码 |
