/* --------------------------------------------------------------------------------------------
 * Copyright (c) Microsoft Corporation. All rights reserved.
 * Licensed under the MIT License. See License.txt in the project root for license information.
 * ------------------------------------------------------------------------------------------ */

import * as fs from 'fs';
import * as path from 'path';
import * as vscode from 'vscode';
import { workspace, ExtensionContext, TextEditorDecorationType, Range } from 'vscode';
import {
    LanguageClient,
    LanguageClientOptions,
    ServerOptions,
    TransportKind
} from 'vscode-languageclient/node';

let client: LanguageClient;
let decorationType: TextEditorDecorationType | null = null;
let myStatusBarItem: vscode.StatusBarItem;
let context: vscode.ExtensionContext;
// let myStatusBarItem: vscode.StatusBarItem;
let spinnerInterval: NodeJS.Timeout | undefined;

// Spinner characters for the animation
const spinnerFrames = ['⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'];
let spinnerIndex = 0;
interface Config {
    fuzz_lsp_path: string;
}

// Function to read the configuration file
function readConfigFile(filePath: string): Config | null {
    try {
        const configFile = fs.readFileSync(filePath, 'utf8');
        const config: Config = JSON.parse(configFile);
        return config;
    } catch (error) {
        console.error('Error reading config file:', error);
        return null;
    }
}

class CustomSidebarViewProvider implements vscode.WebviewViewProvider {
    public static readonly viewType = "fuzzLSPForm";
  
    private _view?: vscode.WebviewView;
  
    constructor(private readonly _extensionUri: vscode.Uri) {
        
    }

    resolveWebviewView(
      webviewView: vscode.WebviewView,
      ctx: vscode.WebviewViewResolveContext<unknown>,
      token: vscode.CancellationToken
    ): void | Thenable<void> {
      this._view = webviewView;
  
      webviewView.webview.options = {
        // Allow scripts in the webview
        enableScripts: true,
        localResourceRoots: [this._extensionUri],
      };
  
      // default webview will show doom face 0
      webviewView.webview.html = this._getHtmlForWebview(webviewView.webview);
      // Listen for messages from the webview
      webviewView.webview.onDidReceiveMessage((message) => {
        switch (message.command) {
            case 'saveSettings':
                this.saveSettings(message.data);
                vscode.window.showInformationMessage('Settings saved successfully!');
                restartLSP(context);
                break;
        }
    });

    }
    
    private saveSettings(config: any) {
        const configFilePath = path.join(this._extensionUri.fsPath, '..\\..\\server_config.json');
        try {
            fs.writeFileSync(configFilePath, JSON.stringify(config, null, 4));
            console.log("Settings Saved in the config file");
        } catch (error) {
            console.error("Failed to save settings:", error);
            vscode.window.showErrorMessage('Failed to save settings.');
        }
    }

    private _getHtmlForWebview(webview: vscode.Webview) {
        const nonce = getNonce();

        return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Configure Fuzz-LSP</title>
    <style>
        :root {
            --background-color-light: #f3f4f6;
            --text-color-light: #333;
            --background-color-dark: #1e1e1e;
            --text-color-dark: #ddd;
        }

        body {
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            background-color: var(--background-color-light);
            color: var(--text-color-light);
            padding: 20px;
            margin: 0;
            transition: background-color 0.3s, color 0.3s;
        }

        h2 {
            color: var(--text-color-light);
            font-size: 24px;
            margin-bottom: 20px;
            transition: color 0.3s;
        }

        .form-group, .radio-group {
            margin-bottom: 20px;
        }

        label {
            display: block;
            font-weight: bold;
            margin-bottom: 5px;
        }

        input[type="text"], input[type="file"] {
            width: calc(100% - 20px);
            padding: 10px;
            margin-top: 5px;
            border: 1px solid #ccc;
            border-radius: 4px;
            box-sizing: border-box;
            font-size: 14px;
            background-color: #fff;
        }

        .radio-group {
            display: flex;
            justify-content: space-around;
            padding: 10px 0;
            background-color: #e2e8f0;
            border-radius: 4px;
            border: 1px solid #ccc;
        }

        .radio-group label {
            margin: 0;
            font-weight: normal;
            font-size: 16px;
            color: #555;
        }

        input[type="radio"] {
            margin-right: 8px;
        }

        button {
            background-color: #4CAF50;
            color: white;
            padding: 10px 20px;
            border: none;
            border-radius: 4px;
            cursor: pointer;
            font-size: 16px;
            transition: background-color 0.3s;
        }

        button:hover {
            background-color: #45a049;
        }

        .form-group.hidden {
            display: none;
        }

        .theme-switch {
            display: flex;
            align-items: center;
            margin-bottom: 20px;
        }

        .theme-switch input {
            margin-right: 10px;
        }
    </style>
</head>
<body>
    <div class="theme-switch">
        <input type="checkbox" id="themeSwitch" name="theme">
        <label for="themeSwitch">Dark Mode</label>
    </div>
    
    <h2>Configure Fuzz-LSP</h2>
    <form id="settingsForm">
        <div class="radio-group">
            <label>
                <input type="radio" id="ollama" name="backend" value="ollama" checked>
                Ollama
            </label>
            <label>
                <input type="radio" id="openai" name="backend" value="openai">
                OpenAI
            </label>
        </div>

        <div class="form-group" id="modelNameGroup">
            <label for="modelName">Model Name:</label>
            <input type="text" id="modelName" name="modelName" required>
        </div>

        <div class="form-group hidden" id="apiKeyGroup">
            <label for="apiKey">API Key:</label>
            <input type="text" id="apiKey" name="apiKey">
        </div>

        <div class="form-group">
            <label for="prompt_file">Choose Prompt File:</label>
            <input type="file" id="prompt_file" name="prompt_file" accept=".txt,.json">
        </div>

        <div class="form-group">
            <label for="retry_prompt">Choose Retry Prompt File:</label>
            <input type="file" id="retry_prompt" name="retry_prompt" accept=".txt,.json">
        </div>

        <button type="button" onclick="submitForm()">Save Settings</button>
    </form>

    <script nonce="${nonce}">
        const vscode = acquireVsCodeApi();

        document.querySelectorAll('input[name="backend"]').forEach(radio => {
            radio.addEventListener('change', function(event) {
                document.getElementById('modelNameGroup').style.display = 'block';
                document.getElementById('apiKeyGroup').style.display = 'none';
            });
        });

        const themeSwitch = document.getElementById('themeSwitch');
        themeSwitch.addEventListener('change', function() {
            if (this.checked) {
                document.body.style.backgroundColor = "var(--background-color-dark)";
                document.body.style.color = "var(--text-color-dark)";
                document.querySelector('h2').style.color = "var(--text-color-dark)";
            } else {
                document.body.style.backgroundColor = "var(--background-color-light)";
                document.body.style.color = "var(--text-color-light)";
                document.querySelector('h2').style.color = "var(--text-color-light)";
            }
        });

        function submitForm() {
            const backend = document.querySelector('input[name="backend"]:checked').value;
            const modelName = document.getElementById('modelName').value;
            const prompt_file = document.getElementById('prompt_file').files[0].path;
            const retry_prompt = document.getElementById('retry_prompt').files[0].path;

            const config = {
                stdio: true,
                version: false,
                backend,
                modelName,
                prompt_file,
                retry_prompt,
                connect_test: false
            };

            vscode.postMessage({ command: 'saveSettings', data: config });
        }
    </script>
</body>
</html>

`;}
}


export function activate(context: vscode.ExtensionContext) {
    const zortikLSPProvider = new CustomSidebarViewProvider(context.extensionUri);
    context.subscriptions.push(
        vscode.window.registerWebviewViewProvider(
          CustomSidebarViewProvider.viewType,
          zortikLSPProvider
        )
    );
    // const myCommandId = 'fuzzLSP.showInfo';
	// context.subscriptions.push(vscode.commands.registerCommand(myCommandId, () => {
	// 	vscode.window.showInformationMessage(`LLM Code Analysis is progress...`);
	// }));
    // myStatusBarItem = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Right, 100);
	// myStatusBarItem.command = myCommandId;
	// context.subscriptions.push(myStatusBarItem);
    // context.subscriptions.push(vscode.window.onDidChangeActiveTextEditor(updateStatusBarItem));
    startLSP(context);
    const myCommandId = 'fuzzLSP.showInfo';
    myStatusBarItem = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Right, 100);
    

    // Start the spinner when extension activates
    startSpinner('Fuzz LSP');
    
    // Show the status bar item
    myStatusBarItem.show();
    

    context.subscriptions.push(vscode.commands.registerCommand(myCommandId, () => {
        vscode.window.showInformationMessage('LLM Code Analysis is in progress...');
    }));
    myStatusBarItem.command = myCommandId;
    context.subscriptions.push(myStatusBarItem);
    // Example usage: Start/Stop spinner based on some action (like server start/stop)
    context.subscriptions.push(vscode.commands.registerCommand('fuzzLSP.startSpinner', () => {
        startSpinner('Fuzz LSP Loading');
    }));

    context.subscriptions.push(vscode.commands.registerCommand('fuzzLSP.stopSpinner', () => {
        stopSpinner('Fuzz LSP Ready');
    }));

    stopSpinner('Fuzz LSP Ready');
}
// Function to start the spinner
function startSpinner(text: string): void {
    if (spinnerInterval) {
        clearInterval(spinnerInterval);
    }
    spinnerIndex = 0;
    myStatusBarItem.text = `${spinnerFrames[spinnerIndex]} ${text}`;

    // Update spinner every 100ms
    spinnerInterval = setInterval(() => {
        spinnerIndex = (spinnerIndex + 1) % spinnerFrames.length;
        myStatusBarItem.text = `${spinnerFrames[spinnerIndex]} ${text}`;
    }, 100);
}

// Function to stop the spinner
function stopSpinner(text: string): void {
    if (spinnerInterval) {
        clearInterval(spinnerInterval);
        spinnerInterval = undefined;
    }
    myStatusBarItem.text = text;  // Set final message when loading is complete
}

function updateStatusBarItem(): void {
	myStatusBarItem.text = `Fuzz LSP`;
	myStatusBarItem.show();
}

function startLSP(context: vscode.ExtensionContext){
    const configFilePath = path.join(__dirname, '../../../config.json');
    const config = readConfigFile(configFilePath);

    if (!config || !config.fuzz_lsp_path) {
        console.error('fuzz_lsp_path is not set in the configuration file');
        return;
    }

    const fusa_lsp_path = config.fuzz_lsp_path;

    // If the extension is launched in debug mode then the debug server options are used
    // Otherwise the run options are used
    const serverOptions: ServerOptions = {
        run: { command: fusa_lsp_path, transport: TransportKind.stdio },
        debug: { command: fusa_lsp_path, transport: TransportKind.stdio }
    };

    // Options to control the language client
    const clientOptions: LanguageClientOptions = {
        // Register the server for plain text documents
        documentSelector: [
            { scheme: 'file', language: 'h' },
            { scheme: 'file', language: 'c' },
            { scheme: 'file', language: 'cpp' },
            { scheme: 'file', language: 'hpp' },
            { scheme: 'file', language: 'python' }
        ],
        synchronize: {
            // Notify the server about file changes to '.clientrc files contained in the workspace
            fileEvents: vscode.workspace.createFileSystemWatcher('**/.clientrc')
        }
    };

    // Create the language client and start the client.
    client = new LanguageClient(
        'FuzzLSP',
        'Fuzz LSP',
        serverOptions,
        clientOptions
    );

    // Start the client. This will also launch the LSP server
    client.start();

    // Listen for the server notification for generated code suggestions
    client.onNotification('window/showGeneratedCode', (params) => handleGeneratedCodeSuggestion(params, context));
    client.onNotification('analysisStarted', (params) => handleSpinner(params, context));
    client.onNotification('analysisDone', (params) => handleSpinner(params, context));
}

function handleSpinner(params: any, context: ExtensionContext){
    if (params == "AnalysisStarted"){
        startSpinner("FuzzLSP Analysis");
    }
    else if (params == "AnalysisDone") {
        stopSpinner("FuzzLSP");
    }
}

function restartLSP(context: vscode.ExtensionContext) {
    if (client) {
        client.stop();
        startLSP(context);
    } else {
        startLSP(context);
    }
}

function getNonce() {
    let text = '';
    const possible = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
    for (let i = 0; i < 32; i++) {
        text += possible.charAt(Math.floor(Math.random() * possible.length));
    }
    return text;
}

// Handle generated code suggestions from the server
function handleGeneratedCodeSuggestion(params: any, context: ExtensionContext) {
    console.log("Notification Received!", params);
    const editor = vscode.window.activeTextEditor;
    if (!editor) {
        return;
    }

    const generatedCode = params;
    let position = editor.selection.active;

    // Clear any existing decoration
    if (decorationType) {
        editor.setDecorations(decorationType, []);
        decorationType.dispose();
        decorationType = null;
    }

    // Create a decoration type for inline code suggestion
    decorationType = vscode.window.createTextEditorDecorationType({
        after: {
            contentText: generatedCode,  // Display the code suggestion
            color: 'rgba(150, 150, 150, 0.7)',  // Light grey color
            fontStyle: 'italic',
        },
        rangeBehavior: vscode.DecorationRangeBehavior.OpenOpen,
    });

    // Create a range that starts at the current cursor position
    const range = new vscode.Range(position, position);

    // Apply the decoration (code suggestion) to the editor
    editor.setDecorations(decorationType, [range]);

    // Listen for the command to accept the suggestion
    const disposable = vscode.commands.registerCommand('fuzzLSP.acceptSuggestion', () => {
        if (decorationType) {
            // Insert the suggestion when the command is triggered
            editor.edit(editBuilder => {
                position = editor.selection.active;
                editBuilder.insert(position, generatedCode);  // Insert the suggestion at the current cursor position
            }).then(() => {
                // Update the cursor position after insertion
                const newPosition = position.translate(0, generatedCode.length);
                editor.selection = new vscode.Selection(newPosition, newPosition);

                // Clear the decoration after the suggestion is inserted
                if (decorationType) {
                    editor.setDecorations(decorationType, []);
                    decorationType.dispose();
                    decorationType = null;
                }
            });
            
            // Clean up the command listener after the suggestion is accepted
            disposable.dispose();
        }
    });

    // Listen for `Ctrl + Right Arrow` key press
    const ctrlRightArrowListener = vscode.commands.registerCommand('cursorWordEndRight', () => {
        if (decorationType) {
            vscode.commands.executeCommand('fuzzLSP.acceptSuggestion');
        } else {
            vscode.commands.executeCommand('cursorWordEndRight'); // Pass through if no suggestion is displayed
        }
    });

    // Store the disposable to clean up later
    context.subscriptions.push(disposable);
    context.subscriptions.push(ctrlRightArrowListener);
}

export function deactivate(): Thenable<void> | undefined {
    if (!client) {
        return undefined;
    }
    // Clean up the decoration when deactivating
    if (decorationType) {
        decorationType.dispose();
        decorationType = null;
    }
    stopSpinner('Fuzz LSP Deactivated');
    return client.stop();
}