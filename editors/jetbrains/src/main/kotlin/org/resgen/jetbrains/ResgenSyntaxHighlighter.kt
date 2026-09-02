package org.resgen.jetbrains

import com.intellij.lexer.Lexer
import com.intellij.openapi.editor.DefaultLanguageHighlighterColors
import com.intellij.openapi.editor.colors.TextAttributesKey
import com.intellij.openapi.fileTypes.SyntaxHighlighterBase
import com.intellij.psi.tree.IElementType

class ResgenSyntaxHighlighter : SyntaxHighlighterBase() {
    companion object {
        // 关键字（橙色）
        val KEYWORD = TextAttributesKey.createTextAttributesKey("RESGEN_KEYWORD", DefaultLanguageHighlighterColors.KEYWORD)
        
        // 路由路径（绿色字符串高亮）
        val ROUTE_PATH = TextAttributesKey.createTextAttributesKey("RESGEN_ROUTE_PATH", DefaultLanguageHighlighterColors.STRING)
        
        // 字段名（经典洋红色/淡紫色键名，与 VSCode entity.name.tag 对齐）
        val FIELD_NAME = TextAttributesKey.createTextAttributesKey("RESGEN_FIELD_NAME", DefaultLanguageHighlighterColors.INSTANCE_FIELD)
        
        // 类型（青色/类名类型，与 VSCode support.type.builtin / variable.other.resgen 对齐）
        val TYPE = TextAttributesKey.createTextAttributesKey("RESGEN_TYPE", DefaultLanguageHighlighterColors.CLASS_NAME)
        
        // 元属性键名（如 wrap=, ctype=）
        val META_ATTR = TextAttributesKey.createTextAttributesKey("RESGEN_META_ATTR", DefaultLanguageHighlighterColors.STATIC_FIELD)
        
        // 常量（如 none, json, form）
        val CONSTANT = TextAttributesKey.createTextAttributesKey("RESGEN_CONSTANT", DefaultLanguageHighlighterColors.CONSTANT)
        
        // 指令/装饰器（@path, @query, @alias，与 VSCode entity.name.function.decorator 对齐）
        val DIRECTIVE = TextAttributesKey.createTextAttributesKey("RESGEN_DIRECTIVE", DefaultLanguageHighlighterColors.METADATA)
        
        // 字符串
        val STRING = TextAttributesKey.createTextAttributesKey("RESGEN_STRING", DefaultLanguageHighlighterColors.STRING)
        
        // 数字
        val NUMBER = TextAttributesKey.createTextAttributesKey("RESGEN_NUMBER", DefaultLanguageHighlighterColors.NUMBER)
        
        // 注释（灰色斜体）
        val COMMENT = TextAttributesKey.createTextAttributesKey("RESGEN_COMMENT", DefaultLanguageHighlighterColors.LINE_COMMENT)
        
        // 操作符与标点
        val OPERATOR = TextAttributesKey.createTextAttributesKey("RESGEN_OPERATOR", DefaultLanguageHighlighterColors.OPERATION_SIGN)
        
        // 普通标识符
        val IDENTIFIER = TextAttributesKey.createTextAttributesKey("RESGEN_IDENTIFIER", DefaultLanguageHighlighterColors.IDENTIFIER)
    }

    override fun getHighlightingLexer(): Lexer = ResgenLexer()

    override fun getTokenHighlights(tokenType: IElementType?): Array<TextAttributesKey> {
        return when (tokenType) {
            ResgenTypes.KEYWORD, ResgenTypes.HTTP_METHOD -> arrayOf(KEYWORD)
            ResgenTypes.ROUTE_PATH -> arrayOf(ROUTE_PATH)
            ResgenTypes.FIELD_NAME -> arrayOf(FIELD_NAME)
            ResgenTypes.TYPE -> arrayOf(TYPE)
            ResgenTypes.META_ATTR -> arrayOf(META_ATTR)
            ResgenTypes.CONSTANT -> arrayOf(CONSTANT)
            ResgenTypes.DIRECTIVE -> arrayOf(DIRECTIVE)
            ResgenTypes.STRING -> arrayOf(STRING)
            ResgenTypes.NUMBER -> arrayOf(NUMBER)
            ResgenTypes.COMMENT -> arrayOf(COMMENT)
            ResgenTypes.OPERATOR -> arrayOf(OPERATOR)
            ResgenTypes.IDENTIFIER -> arrayOf(IDENTIFIER)
            else -> emptyArray()
        }
    }
}
