module UserCenter

# 定义统一响应包装器
wrap ResData<T> {
    code: Int!
    msg: String!
    data: T
}

# 核心用户模型
type User {
    id: Int!
    username: String!
    email: String!
    avatarUrl: String
    createdAt: Time
}

# 装饰器与校验器定义
# 验证邮箱
validator email
# 验证手机号
validator mobile 
# 验证最小长度
validator min(len: Int!)
# 验证最大长度
validator max(len: Int!)

# 限制文件规则：大小、类型、自定义消息
validator fileRule(maxSize: Int!, types: [String!]!, msg: String)

# --- 新增分阶段装饰器定义 ---
decorator loginRequired [stage=request, scope=global]
decorator checkOwner [stage=invoke, scope=specialized]
decorator maskEmail [stage=response, scope=specialized]

# 输入模型
input CreateUserInput {
    # 昵称，长度在 5-20 之间
    username: String! @min(5) @max(20)
    # 邮箱
    email: String! @email
    # 手机号
    mobile: String! @mobile
    # 密码
    password: String!
    # 确认密码
    confirmPassword: String!
}

input UploadAvatarInput {
    id: Int!
    file: File! @fileRule(maxSize: 2097152, types: ["image/jpeg", "image/png"], msg: "头像格式错误或文件太大")
}

input ProfileUpdateInput {
    currentEmail: String! @email
    security: SecurityInfo!
    emailConfirm: String! @email
}

type SecurityInfo {
    backupEmail: String! @email
}

# 接口定义 - 组级配置: wrap=ResData 对组内所有接口生效
group /users [wrap=ResData] {
    @loginRequired
    GET /profile => GetProfile(): ResData<User>

    # 复制用户
    get /user/copy => UserCopy(id: String!): ResData<User>

    POST /register [ctype=form] => CreateUser(input: CreateUserInput): ResData<User>

    @customBind
    @customValidate
    POST /login => Login(username: String, password: String): ResData<Token> [state=201]

    # 测试多阶段装饰器
    @loginRequired
    @checkOwner
    @maskEmail
    POST /update => UpdateUser(id: Int!, email: String): ResData<User>

    POST /avatar [ctype=multipart] => UploadAvatar(input: UploadAvatarInput): ResData<String>

    GET /users => GetUsers(
        page: Int! @query,
        size: Int @query @max(100)
    ): ResData<[User]>

    GET /users/:id => GetUser(id: Int @path): User

    DELETE /users/:id => DeleteUser(id: Int @path): ResData<String>

    POST /update_profile => UpdateProfile(input: ProfileUpdateInput): ResData<String>

    # 导出用户数据, 响应 text/plain, 错误时用 JSON 包装
    GET /export => ExportUsers(): String [ctype=text, etype=json, wrap=ResData]

    GET /test_valid => TestValid(in: ValidGET): Any
}

input ValidGET {
    page: Int
    size: Int
}

type Token {
    token: String!
}
