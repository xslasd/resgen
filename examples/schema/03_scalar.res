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
    startTime: IntTime!
    endTime: IntTime!
}

input QueryEventsInput {
    after: IntTime
    before: IntTime
    page: Int
    size: Int
}

group /events [wrap=ResData] {
    # 标量作为路径参数（@path），由 FromParam 处理字符串解析
    GET /:startTime => GetEventByTime(startTime: IntTime @path): ResData<Event>
    # 标量作为 Query 参数，GET 请求自动展开
    GET /list => ListEvents(input: QueryEventsInput): ResData<[Event]>
    # 标量在请求 Body 中，由序列化层（isolation 模式用 FromValue，direct 模式由用户控制）处理
    POST /create => CreateEvent(input: CreateEventInput): ResData<Event> [state=201]
}
