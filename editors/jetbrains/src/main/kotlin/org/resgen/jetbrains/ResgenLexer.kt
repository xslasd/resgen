package org.resgen.jetbrains

import com.intellij.lexer.LexerBase
import com.intellij.psi.TokenType
import com.intellij.psi.tree.IElementType

class ResgenLexer : LexerBase() {
    private var buffer: CharSequence = ""
    private var startOffset: Int = 0
    private var endOffset: Int = 0
    private var currentOffset: Int = 0
    private var tokenStart: Int = 0
    private var tokenEnd: Int = 0
    private var tokenType: IElementType? = null

    // 记录上一个有意义（非空白、非注释）的 token，以便上下文判定
    private var lastMeaningfulTokenType: IElementType? = null
    private var pendingRoutePath: Boolean = false

    companion object {
        private val KEYWORDS = setOf(
            "module", "type", "input", "wrap", "group", "decorator", "validator", "scalar", "union", "enum", "default"
        )
        private val HTTP_METHODS = setOf(
            "GET", "POST", "PUT", "DELETE", "PATCH", "get", "post", "put", "delete", "patch"
        )
        private val BUILTIN_TYPES = setOf(
            "String", "Int", "Float", "Boolean", "Time", "File", "Any", "Field"
        )
        private val CONSTANTS = setOf(
            "none", "json", "form", "multipart", "xml", "text", "true", "false"
        )
        private val META_ATTR_NAMES = setOf(
            "wrap", "state", "ctype", "etype"
        )
    }

    override fun start(buffer: CharSequence, startOffset: Int, endOffset: Int, initialState: Int) {
        this.buffer = buffer
        this.startOffset = startOffset
        this.endOffset = endOffset
        this.currentOffset = startOffset
        this.lastMeaningfulTokenType = null
        this.pendingRoutePath = false
        advance()
    }

    override fun getState(): Int = 0
    override fun getTokenType(): IElementType? = tokenType
    override fun getTokenStart(): Int = tokenStart
    override fun getTokenEnd(): Int = tokenEnd
    override fun getBufferSequence(): CharSequence = buffer
    override fun getBufferEnd(): Int = endOffset

    override fun advance() {
        if (currentOffset >= endOffset) {
            tokenType = null
            tokenStart = endOffset
            tokenEnd = endOffset
            return
        }

        tokenStart = currentOffset
        val c = buffer[currentOffset]

        // 1. 空白字符
        if (Character.isWhitespace(c)) {
            while (currentOffset < endOffset && Character.isWhitespace(buffer[currentOffset])) {
                currentOffset++
            }
            tokenEnd = currentOffset
            tokenType = TokenType.WHITE_SPACE
            return
        }

        // 2. 注释（以 # 开头直到行尾）
        if (c == '#') {
            while (currentOffset < endOffset && buffer[currentOffset] != '\n' && buffer[currentOffset] != '\r') {
                currentOffset++
            }
            tokenEnd = currentOffset
            tokenType = ResgenTypes.COMMENT
            return
        }

        // 3. 路由路径（紧随 HTTP_METHOD 之后的路径，如 /api/v1/users 或 /users/:id）
        if (pendingRoutePath) {
            pendingRoutePath = false
            // 路由路径字符：通常以 / 开头，或者包含 a-zA-Z0-9_-\/:.
            if (c == '/' || Character.isLetterOrDigit(c) || c == '_' || c == '-' || c == ':') {
                while (currentOffset < endOffset) {
                    val ch = buffer[currentOffset]
                    if (Character.isWhitespace(ch) || ch == '(' || ch == '{' || ch == '=' || ch == '-') {
                        break
                    }
                    currentOffset++
                }
                tokenEnd = currentOffset
                tokenType = ResgenTypes.ROUTE_PATH
                lastMeaningfulTokenType = ResgenTypes.ROUTE_PATH
                return
            }
        }

        // 4. 字符串（以 " 开头）
        if (c == '"') {
            currentOffset++
            while (currentOffset < endOffset) {
                val sc = buffer[currentOffset]
                if (sc == '\\') {
                    currentOffset += 2
                    continue
                }
                if (sc == '"') {
                    currentOffset++
                    break
                }
                if (sc == '\n' || sc == '\r') {
                    break
                }
                currentOffset++
            }
            tokenEnd = currentOffset
            tokenType = ResgenTypes.STRING
            lastMeaningfulTokenType = ResgenTypes.STRING
            return
        }

        // 5. 指令 / 装饰器（以 @ 开头，对应 entity.name.function.decorator）
        if (c == '@') {
            currentOffset++
            while (currentOffset < endOffset && (Character.isLetterOrDigit(buffer[currentOffset]) || buffer[currentOffset] == '_')) {
                currentOffset++
            }
            tokenEnd = currentOffset
            tokenType = ResgenTypes.DIRECTIVE
            lastMeaningfulTokenType = ResgenTypes.DIRECTIVE
            return
        }

        // 6. 数字
        if (Character.isDigit(c)) {
            while (currentOffset < endOffset && (Character.isDigit(buffer[currentOffset]) || buffer[currentOffset] == '.')) {
                currentOffset++
            }
            tokenEnd = currentOffset
            tokenType = ResgenTypes.NUMBER
            lastMeaningfulTokenType = ResgenTypes.NUMBER
            return
        }

        // 7. 标识符及其语义分支（关键字 / HTTP方法 / 字段名 / 元属性 / 类型 / 常量）
        if (Character.isLetter(c) || c == '_') {
            while (currentOffset < endOffset && (Character.isLetterOrDigit(buffer[currentOffset]) || buffer[currentOffset] == '_')) {
                currentOffset++
            }
            tokenEnd = currentOffset
            val word = buffer.subSequence(tokenStart, tokenEnd).toString()

            // 前瞻检查：跳过空白后紧随的是什么字符
            var peekIndex = currentOffset
            while (peekIndex < endOffset && (buffer[peekIndex] == ' ' || buffer[peekIndex] == '\t')) {
                peekIndex++
            }
            val nextChar = if (peekIndex < endOffset) buffer[peekIndex] else null

            val determinedType: IElementType = when {
                // (a) HTTP 方法关键字（GET, POST 等）
                HTTP_METHODS.contains(word) -> {
                    pendingRoutePath = true
                    ResgenTypes.HTTP_METHOD
                }

                // (b) 结构与控制关键字（module, type, input, wrap, group, default 等）
                KEYWORDS.contains(word) -> ResgenTypes.KEYWORD

                // (c) 元属性键名：wrap=, ctype=, etype=, state=
                META_ATTR_NAMES.contains(word) && nextChar == '=' -> ResgenTypes.META_ATTR

                // (d) 模型字段名：field_name: Type
                nextChar == ':' -> ResgenTypes.FIELD_NAME

                // (e) 内置标量类型
                BUILTIN_TYPES.contains(word) -> ResgenTypes.TYPE

                // (f) 语言内置常量（none, json, form, multipart 等）
                CONSTANTS.contains(word) -> ResgenTypes.CONSTANT

                // (g) 自定义类型引用：首字母大写（PascalCase）或者紧跟在冒号/操作符后
                Character.isUpperCase(c) || lastMeaningfulTokenType == ResgenTypes.OPERATOR -> ResgenTypes.TYPE

                else -> ResgenTypes.IDENTIFIER
            }

            tokenType = determinedType
            lastMeaningfulTokenType = determinedType
            return
        }

        // 8. 多字符操作符（->, =>）
        if (c == '-' && currentOffset + 1 < endOffset && buffer[currentOffset + 1] == '>') {
            currentOffset += 2
            tokenEnd = currentOffset
            tokenType = ResgenTypes.OPERATOR
            lastMeaningfulTokenType = ResgenTypes.OPERATOR
            return
        }
        if (c == '=' && currentOffset + 1 < endOffset && buffer[currentOffset + 1] == '>') {
            currentOffset += 2
            tokenEnd = currentOffset
            tokenType = ResgenTypes.OPERATOR
            lastMeaningfulTokenType = ResgenTypes.OPERATOR
            return
        }

        // 9. 单字符操作符/分隔符（:, {, }, (, ), [, ], ,, =, |）
        currentOffset++
        tokenEnd = currentOffset
        tokenType = ResgenTypes.OPERATOR
        lastMeaningfulTokenType = ResgenTypes.OPERATOR
    }
}
