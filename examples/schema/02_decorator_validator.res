# 示例 2：装饰器与校验器
# 展示：
#   - 校验器定义与字段级使用
#   - 装饰器的三个阶段（request / invoke / response）
#   - global（全局）和 specialized（业务模块独立控制）两种作用域
#   - @customBind / @customValidate 逻辑接管
module AuthDemo

# 公共定义（ResData）已在 00_common.res 中声明，此处直接使用

# ── 校验器定义 ──────────────────────────────────────────
validator email
validator mobile
validator min(len: Int!)
validator max(len: Int!)



# ── 装饰器定义（阶段 + 作用域）─────────────────────────
# 全局请求拦截：所有模块共用（如 JWT 鉴权）
decorator auth(role: String!) [stage=request, scope=global]

# 全局请求拦截：登录态检查
decorator loginRequired [stage=request, scope=global]

# 特化调用前拦截：仅由当前 Resolver 独立实现（如资源归属校验）
decorator checkOwner [stage=invoke, scope=specialized]

# 特化响应后处理：仅由当前 Resolver 独立实现（如数据脱敏）
decorator maskEmail [stage=response, scope=specialized]

# 数据模型
type User {
    id:       Int!
    username: String!
    email:    String!
}

type Token {
    token:     String!
    expiresAt: Int!
}

input RegisterInput {
    username: String! @min(3) @max(20)
    email:    String! @email
    mobile:   String! @mobile
    password: String! @min(8)
}

input UpdateInput {
    id:    Int!
    email: String @email
}

group /auth [wrap=ResData] {
    # 注册：使用字段级校验器，请求 form 表单
    POST /register [ctype=form] => Register(input: RegisterInput): ResData<User> [state=201]

    # 登录：完全接管绑定与校验逻辑（业务层手动处理）
    @customBind
    @customValidate
    POST /login => Login(username: String, password: String): ResData<Token>

    # 获取当前用户：全局 loginRequired 装饰器在请求阶段拦截
    @loginRequired
    GET /me => GetMe(): ResData<User>

    # 更新用户：
    #   - loginRequired（request 阶段，全局）：验证登录态
    #   - checkOwner（invoke 阶段，业务独立）：验证是否为资源拥有者
    #   - maskEmail（response 阶段，业务独立）：脱敏邮件地址后再返回
    @loginRequired
    @checkOwner
    @maskEmail
    POST /update => UpdateUser(input: UpdateInput): ResData<User>

    # 管理员接口：需要特定角色，组合多个全局装饰器
    @auth("admin")
    @loginRequired
    DELETE /:id => DeleteUser(id: Int @path): ResData<String>
}
