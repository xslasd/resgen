# 示例 10：单标量 Body 参数自动包装与 Query/Path 提取对比
module TaskDemo

group /tasks {
	# 1. POST 单标量 Body 参数自动合成包装模型
	POST /cancel => CancelTask(task_id: Int!): Boolean

	# 2. PUT 单标量 Body 参数自动合成包装模型
	PUT /name => UpdateName(new_name: String!): Boolean

	# 3. PATCH 单标量 Body 参数自动合成包装模型
	PATCH /status => PatchStatus(status: Int!): Boolean

	# 4. DELETE 单标量 Body 参数自动合成包装模型（与 POST/PUT/PATCH 完全一致）
	DELETE /delete => DeleteTask(task_id: Int!): Boolean

	# 5. 显式 @query 的 POST 参数保持平铺 Query 提取
	POST /trigger => TriggerTask(task_id: Int! @query): Boolean

	# 6. 显式 @query 的 DELETE 参数保持平铺 Query 提取
	DELETE /item => DeleteItem(id: String! @query): Boolean

	# 7. 显式 @path 的 POST 参数保持平铺 Path 提取
	POST /reset/:id => ResetItem(id: Int! @path): Boolean
}
