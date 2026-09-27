#!/usr/bin/env python3
"""Validate the portable ALcli plugin archive before upload or release."""
import json
import struct
import sys
import zipfile
from pathlib import Path

archive = Path(sys.argv[1] if len(sys.argv) > 1 else "dist/alcli-plugin.zip")
required = {
    "plugin/plugin.json",
    "plugin/mcp.json",
    "plugin/.codex-plugin/plugin.json",
    "plugin/.mcp.json",
    "plugin/bin/alcli-mcp",
    "plugin/skills/alcli/SKILL.md",
    "plugin/assets/alcli.png",
}
with zipfile.ZipFile(archive) as z:
    names = set(z.namelist())
    if not required <= names:
        raise SystemExit(f"Missing plugin files: {sorted(required - names)}")
    if any(not name.startswith("plugin/") or ".." in Path(name).parts for name in names):
        raise SystemExit("Archive contains files outside plugin/")
    manifest = json.loads(z.read("plugin/plugin.json"))
    overlay = json.loads(z.read("plugin/.codex-plugin/plugin.json"))
    mcp = json.loads(z.read("plugin/mcp.json"))
    if manifest["name"] != "alcli" or manifest["version"] != overlay["version"]:
        raise SystemExit("Plugin identity or version mismatch")
    if mcp["mcpServers"]["alcli"]["command"] != "./bin/alcli-mcp":
        raise SystemExit("MCP launcher path mismatch")
    if manifest["extensions"]["com.openai"]["interface"]["logo"] != "./assets/alcli.png":
        raise SystemExit("Plugin icon path mismatch")
    skill = z.read("plugin/skills/alcli/SKILL.md").decode()
    if not skill.startswith("---\nname: alcli\n") or "ALCLI_INSTALLER_MANAGED" not in skill:
        raise SystemExit("ALcli skill metadata is missing")
    png = z.read("plugin/assets/alcli.png")
    if png[:8] != b"\x89PNG\r\n\x1a\n":
        raise SystemExit("Icon is not a PNG")
    width, height = struct.unpack(">II", png[16:24])
    if width != height or not 48 <= width <= 4096 or len(png) > 5 * 1024 * 1024:
        raise SystemExit("Plugin icon dimensions or size are invalid")
    launcher = z.getinfo("plugin/bin/alcli-mcp")
    if not launcher.external_attr >> 16 & 0o111:
        raise SystemExit("MCP launcher is not executable")
print(f"Valid ALcli plugin {manifest['version']} with {width}x{height} icon and bundled skill")
