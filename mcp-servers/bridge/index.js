#!/usr/bin/env node

import { spawn } from "node:child_process";
import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import process from "node:process";

import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { z } from "zod";

const workspaceRoot = path.resolve(process.env.OH_MY_BRIDGE_WORKSPACE_ROOT ?? process.cwd());
const codexOutputFile = path.join(os.tmpdir(), "bridge-codex-last.txt");

function resolveCwd(cwd) {
  const target = path.resolve(cwd ? cwd : workspaceRoot);
  const relative = path.relative(workspaceRoot, target);

  if (relative.startsWith("..") || path.isAbsolute(relative)) {
    throw new Error(`cwd must stay within workspace root: ${workspaceRoot}`);
  }

  return target;
}

function getProvider(model) {
  if (model.startsWith("gemini-")) {
    return "gemini";
  }

  if (
    model.startsWith("gpt-") ||
    model.startsWith("codex-") ||
    model.startsWith("o")
  ) {
    return "codex";
  }

  throw new Error(
    "Unsupported model prefix. Use gemini-* or gpt-*/codex-*/o* models.",
  );
}

async function runGemini({ prompt, cwd, model, timeoutMs }) {
  const args = ["-m", model, "-p", prompt, "--yolo"];

  return await runCli({
    command: "gemini",
    args,
    cwd,
    timeoutMs,
    errorPrefix: "Gemini CLI",
  });
}

async function runCodex({ prompt, cwd, model, reasoningEffort, timeoutMs }) {
  const args = [
    "exec",
    "-m",
    model,
    "--dangerously-bypass-approvals-and-sandbox",
    "-o",
    codexOutputFile,
  ];

  if (reasoningEffort) {
    args.push("--config", `model_reasoning_effort=${reasoningEffort}`);
  }

  args.push("-C", cwd, prompt);

  const result = await runCli({
    command: "codex",
    args,
    cwd,
    timeoutMs,
    errorPrefix: "Codex CLI",
  });

  if (result.text) {
    return result;
  }

  try {
    const fileOutput = (await fs.readFile(codexOutputFile, "utf8")).trim();
    if (fileOutput) {
      return { text: fileOutput };
    }
  } catch {
    // Ignore missing or unreadable fallback output file and fail below.
  }

  return { text: "(done)" };
}

async function runCli({ command, args, cwd, timeoutMs, errorPrefix }) {
  return await new Promise((resolve, reject) => {
    const child = spawn(command, args, {
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
      reject(new Error(`${errorPrefix} timed out after ${timeoutMs}ms`));
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
        reject(new Error(`${errorPrefix} exited with code ${code}: ${detail}`));
        return;
      }

      resolve({ text: stdout.trim() });
    });
  });
}

const server = new McpServer({
  name: "oh-my-bridge-bridge",
  version: "1.0.0",
});

server.tool(
  "delegate",
  "Delegate a code generation task to the best available AI model.",
  {
    prompt: z.string().min(1),
    model: z.string().describe(
      "Target model. Supported prefixes include gemini-2.5-pro, gemini-2.5-flash, gpt-5.4, gpt-5.3-codex, o3, and related gemini-*/gpt-*/codex-*/o* models.",
    ),
    cwd: z.string().optional(),
    timeoutMs: z.number().int().positive().max(300000).optional(),
    reasoning_effort: z.string().optional(),
  },
  async ({ prompt, model, cwd, timeoutMs, reasoning_effort: reasoningEffort }) => {
    const provider = getProvider(model);
    const resolvedCwd = resolveCwd(cwd);
    const result =
      provider === "gemini"
        ? await runGemini({
            prompt,
            cwd: resolvedCwd,
            model,
            timeoutMs: timeoutMs ?? 120000,
          })
        : await runCodex({
            prompt,
            cwd: resolvedCwd,
            model,
            reasoningEffort,
            timeoutMs: timeoutMs ?? 180000,
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
        model,
        provider,
      },
    };
  },
);

const transport = new StdioServerTransport();
await server.connect(transport);
