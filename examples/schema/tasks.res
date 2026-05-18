module TaskCenter

validator timeBefore(field: Field!)

input CreateTaskInput {
    title: String!
    period: TaskPeriod!
}

input TaskPeriod {
    startTime: IntTime! @timeBefore("endTime")
    endTime: IntTime!
}

input GetTasksInput {
    page: Int
    size: Int
}

group /tasks [wrap=ResData] {
    POST /create => CreateTask(input: CreateTaskInput): ResData<String>!

    GET /list => GetTasks(input: GetTasksInput): ResData<String>!
}
