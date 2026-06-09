import * as vscode from 'vscode';
import * as fs from 'fs';
import * as path from 'path';
import {
    LanguageClient,
    LanguageClientOptions,
    ServerOptions,
    TransportKind
} from 'vscode-languageclient/node';

let client: LanguageClient;

export function activate(context: vscode.ExtensionContext) {
    // 获取配置中的 resgen.path。如果未配置，默认使用环境变量 "resgen"
    const config = vscode.workspace.getConfiguration('resgen');
    const binPath = config.get<string>('path') || 'resgen';

    // 记录和展示诊断信息
    const outputChannel = vscode.window.createOutputChannel('Resgen LSP');
    outputChannel.appendLine(`[Resgen] Activating extension...`);
    outputChannel.appendLine(`[Resgen] Configured path: ${binPath}`);

    // 检查文件是否存在
    if (binPath !== 'resgen') {
        const resolvedPath = path.resolve(binPath);
        if (!fs.existsSync(resolvedPath)) {
            vscode.window.showErrorMessage(`Resgen LSP: 可执行文件未找到: ${resolvedPath}，请检查 .vscode/settings.json 中的 resgen.path 配置！`);
            outputChannel.appendLine(`[Resgen] Error: Executable not found at ${resolvedPath}`);
            return;
        } else {
            outputChannel.appendLine(`[Resgen] Executable verified at ${resolvedPath}`);
        }
    }

    // 语言服务参数，通过 stdio 与 resgen lsp 进行交互
    const serverOptions: ServerOptions = {
        command: binPath,
        args: ['lsp'],
        transport: TransportKind.stdio
    };

    // 客户端选项
    const clientOptions: LanguageClientOptions = {
        // 注册关联的文件后缀
        documentSelector: [{ scheme: 'file', language: 'resgen' }],
        synchronize: {
            // 当工作区中的配置文件发生变化时同步
            fileEvents: vscode.workspace.createFileSystemWatcher('**/*.res')
        },
        outputChannel: outputChannel
    };

    // 创建并启动客户端
    client = new LanguageClient(
        'resgenLSP',
        'Resgen Language Server',
        serverOptions,
        clientOptions
    );

    // 启动客户端后，它会自动与语言服务端建立连接
    client.start();
    outputChannel.appendLine(`[Resgen] Language Client started.`);
}

export function deactivate(): Thenable<void> | undefined {
    if (!client) {
        return undefined;
    }
    return client.stop();
}
