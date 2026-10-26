# 自定义标量系统 (Scalars)

在真实的业务系统开发中，基础类型（如 `Int`、`String`）往往不足以表达丰富的领域语义。例如：一个整数时间戳在业务代码中更适合作为 `time.Time` 处理；一个敏感 ID 在业务中应该被包装为具有校验能力的自定义类型。

Resgen 提供了强大的**自定义标量系统 (Scalar)**，允许开发者定义传输层基础格式与业务物理类型之间的无缝映射。

---

## 1. 标量 DSL 语法

### 1.1 显式继承基础类型
在冒号后指定网络传输层的物理基础类型（如 `Int`、`String`、`Float`）：

```graphql
# 网络传输层使用 Int64 整数，业务层映射为自定义时间对象
scalar IntTime: Int

# 网络传输层使用 String 字符串，业务层映射为 UUID 或货币对象
scalar Decimal: String
```

### 1.2 省略基础类型（默认使用 String）
若未显式指定基础类型，默认以 `String` 作为物理传输载体：

```graphql
# 默认使用 String 传输
scalar OrderStatus
```

### 1.3 逃生舱标量 (Dynamic Any)
当传输层是一个动态无固定 Schema 的 JSON 结构时，可以继承自 `Any`：

```graphql
scalar DynamicSetting: Any
```

> [!CAUTION]
> **严禁继承 `File` 类型**：自定义标量**绝对不能**声明为 `scalar MyFile: File`。`File` 属于专用于文件流的内置复合管道，无法作为标量的网络传输载体。

---

## 2. 两种代码生成风格 (`scalar_style`)

Resgen 支持两种截然不同的标量生成策略，可在 `resgen.yaml` 中一键切换：

```yaml
generator:
  scalar_style: "isolation" # 方案 A：DTO 隔离流 (默认)
  # scalar_style: "direct"  # 方案 B：标量直通流
```

### 2.1 方案 A：`isolation`（DTO 隔离流）—— 推荐默认

**设计哲学**：传输层与业务领域层彻底解耦。
- **生成的业务实体**：直接使用最地道、最方便的原生 Go 类型（例如 `time.Time`）；
- **传输层**：生成器在底层额外合成一个轻量的 `DTO` 结构体，由框架自动调用 `ToDTO/FromDTO` 完成转换，业务层完全无感。

```go
// 业务模型：直接使用原生类型，开箱即用，支持各种标准库生态
type EventOutput struct {
    EventID   int        `json:"event_id"`
    StartTime *time.Time `json:"start_time"`
}

// 底层自动生成的 DTO 隔离模型（由框架在网络边界自动装配，业务无需关心）
type EventOutputDTO struct {
    EventID   int    `json:"event_id"`
    StartTime *int64 `json:"start_time"`
}
```

#### Go 侧标量实现契约
在 `isolation` 模式下，自定义标量需要实现以下三组标准方法：

| 方法 | 接收者 | 方法签名 | 核心职责 |
|:---|:---|:---|:---|
| **`FromParam`** | 指针接收者 | `FromParam(ctx any, s string) error` | 从 URL Path、Query、Header 提取的字符串中解析。 |
| **`FromValue`** | 指针接收者 | `FromValue(ctx any, v BaseType) error` | 从 Request Body 反序列化后的基础标量（如 `int64`）赋值给对象。 |
| **`ToValue`** | 值接收者 | `ToValue(ctx any) (BaseType, error)` | 将业务对象转换为网络基础类型（如 `int64`），准备写入响应。 |

> [!IMPORTANT]
> `FromValue` 的入参类型与 `ToValue` 的出参类型，**必须**与 DSL 中声明的基础类型严格对应（如 `scalar IntTime: Int` 对应 Go 的 `int64`）。Resgen 编译器会在编译期执行 AST 静态分析，类型不匹配时会主动拦截并报错。

#### `isolation` 完整工程代码示范
```go
package scalars

import (
    "strconv"
    "time"
)

type IntTime time.Time

// FromParam: 解析字符串路径/查询参数 (string -> IntTime)
func (it *IntTime) FromParam(ctx any, s string) error {
    sec, err := strconv.ParseInt(s, 10, 64)
    if err != nil {
        return err
    }
    *it = IntTime(time.Unix(sec, 0))
    return nil
}

// FromValue: 解析 Body 基础值 (int64 -> IntTime)
func (it *IntTime) FromValue(ctx any, v int64) error {
    *it = IntTime(time.Unix(v, 0))
    return nil
}

// ToValue: 序列化为网络基础值 (IntTime -> int64)
func (it IntTime) ToValue(ctx any) (int64, error) {
    return time.Time(it).Unix(), nil
}
```

---

### 2.2 方案 B：`direct`（标量直通流）

**设计哲学**：追求极致精简的代码量。
- 不生成任何 DTO 结构体；
- 业务模型中直接引用标量类型别名本身（例如 `*scalars.IntTime`）；
- 序列化行为交由标准库的 `MarshalJSON / UnmarshalJSON`（或 MsgPack 等）自行处理。

```go
type EventOutput struct {
    EventID   int              `json:"event_id"`
    StartTime *scalars.IntTime `json:"start_time"` // 直接使用别名
}
```

#### Go 侧标量实现契约
仅需实现 `FromParam`，Body 编解码使用标准 JSON 接口：

```go
package scalars

import (
    "encoding/json"
    "strconv"
    "time"
)

type IntTime time.Time

func (it *IntTime) FromParam(ctx any, s string) error {
    sec, err := strconv.ParseInt(s, 10, 64)
    if err != nil {
        return err
    }
    *it = IntTime(time.Unix(sec, 0))
    return nil
}

// 自行实现标准 JSON 序列化
func (it IntTime) MarshalJSON() ([]byte, error) {
    return json.Marshal(time.Time(it).Unix())
}

// 自行实现标准 JSON 反序列化
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

## 3. 两种生成风格横向对比选型

| 评估维度 | `isolation` (DTO 隔离流) | `direct` (标量直通流) |
|:---|:---|:---|
| **业务层体验** | 🌟🌟🌟🌟🌟 原生 Go 类型（如 `*time.Time`），完美融入生态 | 🌟🌟🌟 标量别名类型（如 `*scalars.IntTime`） |
| **生成代码体积** | 较多（包含 DTO 和 ToDTO/FromDTO 方法） | 🌟🌟🌟🌟🌟 极简（无任何中间结构体） |
| **上下文感知 (`ctx`)**| ✅ 支持在转换时传递 `ctx`（可实现基于当前请求时区的动态时间转换） | ❌ 不支持（标准序列化方法无 `ctx` 参数） |
| **AST 强类型检查** | ✅ 编译期强校验 `ToValue/FromValue` 类型匹配度 | ❌ 由 JSON 库在运行时反射决定 |
| **适用场景** | 复杂业务系统、企业级中后台、需要动态时区/汇率转换 | 轻量微服务、追求极致精简或已有完善的第三方自定义类型库 |

---

## 4. `resgen.yaml` 映射配置

在配置文件中将 DSL 标量名映射至 Go 物理包路径：

```yaml
scalars:
  IntTime:
    model: "github.com/your-org/project/pkg/scalars.IntTime"
```

Resgen 编译器通过动态 AST 分析自动提取包名和底层结构，无需开发者手动指定目标类型，真正做到零多余配置。
