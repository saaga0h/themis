#!/bin/bash
# Install workflow agents and commands globally for Claude Code
set -euo pipefail

AGENT_DIR="$HOME/.claude/agents"
COMMAND_DIR="$HOME/.claude/commands"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "Installing workflow agents and commands..."

mkdir -p "$AGENT_DIR"
mkdir -p "$COMMAND_DIR"

# Install agents
for agent in "$SCRIPT_DIR"/agents/*.md; do
    name=$(basename "$agent")
    if [ -f "$AGENT_DIR/$name" ]; then
        echo "  ⚠ $AGENT_DIR/$name exists, backing up to $name.bak"
        cp "$AGENT_DIR/$name" "$AGENT_DIR/$name.bak"
    fi
    cp "$agent" "$AGENT_DIR/$name"
    echo "  ✓ Agent: $name"
done

# Install commands
for cmd in "$SCRIPT_DIR"/commands/*.md; do
    name=$(basename "$cmd")
    if [ -f "$COMMAND_DIR/$name" ]; then
        echo "  ⚠ $COMMAND_DIR/$name exists, backing up to $name.bak"
        cp "$COMMAND_DIR/$name" "$COMMAND_DIR/$name.bak"
    fi
    cp "$cmd" "$COMMAND_DIR/$name"
    echo "  ✓ Command: $name"
done

echo ""
echo "Done. Installed:"
echo "  Agents:   $(ls "$SCRIPT_DIR"/agents/*.md | wc -l | tr -d ' ') → $AGENT_DIR/"
echo "  Commands: $(ls "$SCRIPT_DIR"/commands/*.md | wc -l | tr -d ' ') → $COMMAND_DIR/"
echo ""
echo "Restart Claude Code to pick up new agents and commands."
echo "Commands available: /architect, /implement, /review, /ship, /context, /intent-bridge, /feature, /concept, /document"
echo ""
echo "Quick start — TDD-first:"
echo "  cd your-project"
echo "  /feature describe what you want to build      # interview → tests → architect → implement"
echo "  /concept path/to/intent-doc.md                # from web conversation → same pipeline"
echo ""
echo "Quick start — direct:"
echo "  cd your-project"
echo "  /architect describe what you want to build"
echo "  /implement plan-name"
echo "  /review [--quick|--security|--architecture|--complexity|--conventions|--coverage]"
echo "  /ship plan-name"
