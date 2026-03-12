#!/usr/bin/env node

import { spawn } from "node:child_process";
import path from "node:path";
import process from "node:process";

import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { z } from "zod";

const workspaceRoot = path.resolve(process.env.OH_MY_BRIDGE_WORKSPACE_ROOT ?? process.cwd());

function resolveCwd(cwd) {
  const target = path.resolve(cwd ? cwd : workspaceRoot);
  const relative = path.relative(workspaceRoot, target);

  if (relative.startsWith("..") || path.isAbsolute(relative)) {
    throw new Error(`cwd must stay within workspace root: ${workspaceRoot}`);
  }

  return target;
}

async function runGemini({ prompt, cwd, model, timeoutMs }) {
  const args = ["-p", prompt, "--yolo"];
  if (model) {
    args.push("-m", model);
  }

  return await new Promise((resolve, reject) => {
    const child = spawn("gemini", args, {
      cwd,
      env: process.env,
      stdio: ["ignore", "pipe", "pipe"],
    });

    let stdout = "";
    let stderr = "";
    let settled = false;

    const timer = setTimeout(() => {
      settled = true;
      child.kill("SIGTERM");
      reject(new Error(`Gemini CLI timed out after ${timeoutMs}ms`));
    }, timeoutMs);

    child.stdout.setEncoding("utf8");
    child.stderr.setEncoding("utf8");
    child.stdout.on("data", (chunk) => {
      stdout += chunk;
    });
    child.stderr.on("data", (chunk) => {
      stderr += chunk;
    });

    child.on("error", (error) => {
      if (settled) {
        return;
      }

      settled = true;
      clearTimeout(timer);
      reject(error);
    });

    child.on("close", (code, signal) => {
      if (settled) {
        return;
      }

      settled = true;
      clearTimeout(timer);

      if (code !== 0) {
        const detail = stderr.trim() || stdout.trim() || `signal=${signal ?? "none"}`;
        reject(new Error(`Gemini CLI exited with code ${code}: ${detail}`));
        return;
      }

      resolve({ text: stdout.trim() || "(done)" });
    });
  });
}

const server = new McpServer({
  name: "oh-my-bridge-gemini",
  version: "0.1.0",
});

server.tool(
  "gemini",
  "Run Gemini CLI with project-aware cwd and return the text response.",
  {
    prompt: z.string().min(1),
    cwd: z.string().optional(),
    model: z.string().refine(
      (m) => !m || /gemini-2\.5|gemini-3/.test(m),
      { message: "gemini-2.0 and older models do not support thinking config required by gemini-cli. Use gemini-2.5-flash or gemini-2.5-pro." }
    ),
    sandbox: z.string().optional(),
    "approval-policy": z.string().optional(),
    timeoutMs: z.number().int().positive().max(300000).optional(),
  },
  async ({ prompt, cwd, model, sandbox, "approval-policy": approvalPolicy, timeoutMs  }) => {
    void sandbox; // accepted by MCP schema for API compatibility but not used by Gemini CLI
    void approvalPolicy; // accepted by MCP schema for API compatibility but not used by Gemini CLI

    const resolvedCwd = resolveCwd(cwd);
    const result = await runGemini({
      prompt,
      cwd: resolvedCwd,
      model,
      timeoutMs: timeoutMs ?? 120000,
    });

    return {
      content: [
        {
          type: "text",
          text: result.text,
        },
      ],
      structuredContent: {
        response: result.text,
        cwd: resolvedCwd,
        model: model ?? null,
      },
    };
  },
);

const transport = new StdioServerTransport();
await server.connect(transport);
