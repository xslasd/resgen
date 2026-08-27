# 示例 3：自定义标量的使用
# 展示：
# - scalar 声明语法（DSL BaseType 映射）
# - 在模型字段、路径参数、Query 参数、请求 Body 中使用标量
# - 嵌套结构体中包含标量字段
module ScalarDemo

# 使用标量作为模型字段
type Event {
    id: Int!
    name: String!
    startTime: IntTime!
    endTime: IntTime
    createdAt: IntTime
}

input CreateEventInput {
    name: String!
    # 使用 @alias 自定义传输别名，兼容老前端/老系统不规范命名
    startTime: IntTime! @alias("st_time")
    endTime: IntTime!   @alias("end_time")
}

input QueryEventsInput {
    after: IntTime  @alias("from_tm")
    before: IntTime @alias("to_tm")
    page: Int
    size: Int
}

group /events {
    # 标量作为路径参数（@path），由 FromParam 处理字符串解析
    @auth("events:get")
    GET /:startTime => GetEventByTime(startTime: IntTime @path): Event
    # 标量作为 Query 参数，GET 请求自动展开
    @auth("events:list")
    GET /list => ListEvents(input: QueryEventsInput): [Event]
    # 标量在请求 Body 中，由序列化层处理
    POST /create => CreateEvent(input: CreateEventInput): Event [state=201]
}
