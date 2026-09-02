package org.resgen.jetbrains

import com.intellij.execution.configurations.GeneralCommandLine
import com.intellij.openapi.project.Project
import com.intellij.openapi.vfs.VirtualFile
import com.intellij.platform.lsp.api.LspServerSupportProvider
import com.intellij.platform.lsp.api.ProjectWideLspServerDescriptor

class ResgenLspServerSupportProvider : LspServerSupportProvider {
    override fun fileOpened(project: Project, file: VirtualFile, serverStarter: LspServerSupportProvider.LspServerStarter) {
        val ext = file.extension
        if (ext == "res" || ext == "resgen" || file.fileType is ResgenFileType) {
            serverStarter.ensureServerStarted(ResgenLspServerDescriptor(project, "Resgen LSP"))
        }
    }
}

class ResgenLspServerDescriptor(project: Project, presentableName: String) : ProjectWideLspServerDescriptor(project, presentableName) {
    override fun isSupportedFile(file: VirtualFile): Boolean {
        val ext = file.extension
        return ext == "res" || ext == "resgen" || file.fileType is ResgenFileType
    }

    override fun createCommandLine(): GeneralCommandLine {
        return GeneralCommandLine("resgen", "lsp")
    }
}
