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
- `@form`: 表单字段
- `@customBind`: **逻辑接管绑定**。跳过自动生成代码的绑定逻辑，在 Resolver 中生成 `BindXXX` 方法供手动实现（适用于 Multipart 复杂处理或自定义解析）。
- 默认逻辑：
  - `GET` 方法：参数默认绑定到 `Query`。
  - `POST/PUT/PATCH` 方法：参数默认绑定到 `Body` (JSON) 或 Form。

### 参数规范
- **GET 扁平化**：针对 `GET` 接口，如果参数是结构体，Resgen 会自动将其字段展开为顶层查询参数（如 `?page=1&size=10`）。
- **禁止嵌套**：`GET` 接口的结构体参数不允许包含嵌套结构体。

## 3. 装饰器与校验器

- **装饰器 (Decorator)**：用于注入中间件逻辑（如认证）。
- **校验器 (Validator)**：用于字段校验。
GET /check => CheckEmail(email: String @email): Any

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

### API 文档配置
- **`base_url`**: 文档中显示的 API 基准请求地址。
- **`doc_case`**: 统一 API 文档中呈现的字段命名风格。

### 默认响应行为
- **`default_ok_status`**: 接口未定义 `state` 时默认的成功状态码。
- **`default_wrap`**: 接口未定义 `wrap` 时默认应用到的全局错误包装器。
