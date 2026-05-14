module TaskCenter

input CreateTaskInput {
    title: String!
    period: TaskPeriod!
}

input TaskPeriod {
    startTime: Time!
    endTime: Time!
}

input GetTasksInput {
    page: Int
    size: Int
}

group /tasks [wrap=ResData] {
    POST /create => CreateTask(input: CreateTaskInput): ResData<String>!

    GET /list => GetTasks(input: GetTasksInput): ResData<String>!
}
