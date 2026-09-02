package org.resgen.jetbrains

import com.intellij.psi.tree.IElementType

class ResgenTokenType(debugName: String) : IElementType(debugName, ResgenLanguage)

object ResgenTypes {
    @JvmField val KEYWORD = ResgenTokenType("KEYWORD")
    @JvmField val HTTP_METHOD = ResgenTokenType("HTTP_METHOD")
    @JvmField val ROUTE_PATH = ResgenTokenType("ROUTE_PATH")
    @JvmField val FIELD_NAME = ResgenTokenType("FIELD_NAME")
    @JvmField val META_ATTR = ResgenTokenType("META_ATTR")
    @JvmField val TYPE = ResgenTokenType("TYPE")
    @JvmField val CONSTANT = ResgenTokenType("CONSTANT")
    @JvmField val DIRECTIVE = ResgenTokenType("DIRECTIVE")
    @JvmField val STRING = ResgenTokenType("STRING")
    @JvmField val NUMBER = ResgenTokenType("NUMBER")
    @JvmField val COMMENT = ResgenTokenType("COMMENT")
    @JvmField val OPERATOR = ResgenTokenType("OPERATOR")
    @JvmField val IDENTIFIER = ResgenTokenType("IDENTIFIER")
}
