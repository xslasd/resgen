package org.resgen.jetbrains

import com.intellij.openapi.fileTypes.LanguageFileType
import com.intellij.openapi.util.IconLoader
import javax.swing.Icon

class ResgenFileType private constructor() : LanguageFileType(ResgenLanguage) {
    companion object {
        @JvmStatic
        val INSTANCE = ResgenFileType()
    }

    override fun getName(): String = "Resgen"

    override fun getDescription(): String = "Resgen DSL file"

    override fun getDefaultExtension(): String = "resgen"

    override fun getIcon(): Icon = IconLoader.getIcon("/icons/resgen.svg", ResgenFileType::class.java)
}
