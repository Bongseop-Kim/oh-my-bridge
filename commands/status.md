---
name: oh-my-bridge:status
description: Show current config routes, model definitions, and CLI availability
---

# oh-my-bridge Status

Call `mcp__bridge__status` and display the result.

## Steps

1. **Call the status tool**

Call `mcp__bridge__status` with no arguments.

2. **Display the result**

Format the response as follows:

```text
## oh-my-bridge status

Config: <config_path>
Note: CLI status reflects server startup snapshot. Restart Claude Code after installing/removing CLIs.

### Routes
| Category | Model | CLI |
|----------|-------|-----|
| <category> | <model> | ✓ installed / ✗ not found / — (claude) |
...

### Models
| Model | Command | Args |
|-------|---------|------|
| <model> | <command> | <args joined by space> |
...
```

For the Routes table:
- If the route value is `"claude"`, show `—` in the CLI column
- Otherwise look up the model's `command` in `cli_status` and show `✓ installed` or `✗ not found`

3. **If the tool call fails**

Tell the user:
- MCP server may not be running — check `/mcp` to confirm `bridge · ✔ connected`
- If disconnected, restart Claude Code and try again
