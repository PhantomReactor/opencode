import { Log } from "../util/log"
import { Global } from "../global"
import { App } from "../app/app"
import fs from "fs/promises"
import path from "path"
import { spawn } from "child_process"

export namespace Commands {
  const log = Log.create({ service: "commands" })

  export interface CustomCommand {
    name: string
    description: string
    prompt: string
    variables: string[]
  }

  export interface ExecuteRequest {
    prompt: string
    variables: Record<string, string>
  }

  export interface ExecuteResponse {
    processedPrompt: string
  }

  export async function list(): Promise<CustomCommand[]> {
    const commandsDir = path.join(Global.Path.config, "commands")
    const commandsMap = new Map<string, CustomCommand>()

    async function readCommandsRecursively(dir: string): Promise<void> {
      try {
        const entries = await fs.readdir(dir, { withFileTypes: true })

        for (const entry of entries) {
          const fullPath = path.join(dir, entry.name)

          if (entry.isDirectory()) {
            await readCommandsRecursively(fullPath)
          } else if (entry.isFile() && entry.name.endsWith(".md")) {
            const content = await fs.readFile(fullPath, "utf-8")
            const commandName = entry.name.replace(".md", "")

            const varRegex = /\$(\w+)/g
            const variables: string[] = []
            let match
            while ((match = varRegex.exec(content)) !== null) {
              if (!variables.includes(match[1])) {
                variables.push(match[1])
              }
            }

            commandsMap.set(commandName, {
              name: commandName,
              description: `custom command: ${commandName}`,
              prompt: content,
              variables,
            })
          }
        }
      } catch (error) {
        log.debug("Failed to read commands directory", { dir, error })
      }
    }
    await readCommandsRecursively(commandsDir)
    return Array.from(commandsMap.values())
  }

  export async function execute(request: ExecuteRequest): Promise<ExecuteResponse> {
    const { prompt, variables } = request

    let processedPrompt = prompt
    for (const [key, value] of Object.entries(variables)) {
      processedPrompt = processedPrompt.replaceAll(`$${key}`, value)
    }

    const terminalCommandRegex = /!\`([^`]+)\`/g
    const matches = Array.from(processedPrompt.matchAll(terminalCommandRegex))

    for (const match of matches) {
      const fullMatch = match[0]
      const command = match[1]

      try {
        const output = await executeTerminalCommand(command)
        processedPrompt = processedPrompt.replace(fullMatch, output)
      } catch (error) {
        const errorMsg = error instanceof Error ? error.message : "Command execution failed"
        processedPrompt = processedPrompt.replace(fullMatch, `[Error: ${errorMsg}]`)
      }
    }

    return { processedPrompt }
  }

  async function executeTerminalCommand(command: string): Promise<string> {
    return new Promise<string>((resolve, reject) => {
      const [cmd, ...args] = command.split(" ")
      const child = spawn(cmd, args, {
        cwd: App.info().path.cwd,
        stdio: ["pipe", "pipe", "pipe"],
      })

      let stdout = ""
      let stderr = ""

      child.stdout.on("data", (data) => {
        stdout += data.toString()
      })

      child.stderr.on("data", (data) => {
        stderr += data.toString()
      })

      child.on("close", (code) => {
        if (code === 0) {
          resolve(stdout.trim())
        } else {
          reject(new Error(`Command failed with code ${code}: ${stderr}`))
        }
      })

      child.on("error", (error) => {
        reject(error)
      })
    })
  }
}
