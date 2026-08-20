# 示例：枚举特性演示
module EnumDemo

# 1. 基础 String 类型枚举
enum UserRole : String {
	ADMIN : "admin"
	USER : "user"
	GUEST : "guest"
}

# 2. 基础 Int 类型枚举
enum RecordStatus : Int {
	# 禁用状态
	DISABLE : 0
	
	# 启用状态
	ENABLE : 1
}

# 3. 继承自 Scalar 并且智能退化
scalar Timestamp : Int

# 由于 Timestamp 退化为了 int64，系统会自动将这个枚举映射为 int64
enum SpecialTime : Timestamp {
	EPOCH : 0
	Y2K   : 946684800
}

type UserWithRole {
	id: String
	role: UserRole
	status: RecordStatus
	createdAt: SpecialTime
}

input CreateUserInput {
	role: UserRole!
	status: RecordStatus!
	createdAt: SpecialTime
}

group EnumGroup /enum {
	# 创建带有枚举的用户
	POST /create => CreateUser(input: CreateUserInput!): UserWithRole

	# 通过参数查询
	GET /query => QueryByRole(role: UserRole @query, status: RecordStatus @query): UserWithRole
}
