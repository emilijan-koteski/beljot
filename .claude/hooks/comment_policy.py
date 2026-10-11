#!/usr/bin/env python3
"""Rejects code comments added by Claude Code agents unless they are a short doc comment.

Allowed: a comment of at most three lines and two sentences directly above a function,
method, or top-level type declaration, plus tool directives (go:, eslint-, @ts-, nolint...).
Only comments that were not already in the file are reported.

Modes:
  pretool  PreToolUse hook for Write/Edit/MultiEdit; denies the call.
  stop     Stop/SubagentStop hook; scans uncommitted changes against HEAD and blocks once.
  check    Manual run over uncommitted changes; exits 1 on violations.
  audit    Lists every comment outside the policy in the given paths (default: whole repo); --lines shows each one.
"""
import json
import os
import re
import subprocess
import sys
from collections import Counter

CODE_EXTENSIONS = (".go", ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs")
SKIPPED_DIRS = ("node_modules/", "_bmad/", "_bmad-output/", ".claude/", "dist/", "build/", "vendor/")
MAX_DOC_LINES = 3
MAX_DOC_SENTENCES = 2

DIRECTIVE = re.compile(
    r"^(go:|nolint|\+build|eslint-|@ts-|prettier-ignore|biome-ignore|istanbul |c8 |#region|#endregion"
    r"|Code generated|/? ?<reference|@vite-ignore|webpackChunkName|#__PURE__|@__PURE__|@jsx)"
)
STRING_LITERAL = re.compile(r'"(?:\\.|[^"\\])*"|\'(?:\\.|[^\'\\])*\'|`(?:\\.|[^`\\])*`')
TS_MODIFIERS = re.compile(r"^(export\s+|default\s+|declare\s+|async\s+|abstract\s+)*")
ARROW_FUNCTION = r"(const|let)\s+\w+\s*(:[^=]+)?=\s*(async\s+)?(\(.*\)\s*(:[^=]+)?=>|\($|<|\w+\s*=>|function\b|(React\.)?(memo|forwardRef)(<.*>)?\()"
TS_TOP_DECL = re.compile(r"^(function\b|class\b|interface\b|type\s+\w+|enum\b|" + ARROW_FUNCTION + ")")
TS_NESTED_FUNCTION = re.compile(r"^\s+(async\s+)?(function\b|" + ARROW_FUNCTION + ")")
TS_METHOD = re.compile(
    r"^(  |\t|    )((public|private|protected|static|async|readonly|override|get|set)\s+)*"
    r"#?\*?(\w+)\s*(<[^>]*>)?\s*\(.*\{\s*$"
)
TS_ARROW_PROPERTY = re.compile(r"^(  |\t|    )((public|private|protected|static|readonly)\s+)*\w+\s*=\s*(async\s+)?\(")
CONTROL_KEYWORDS = {"if", "for", "while", "switch", "catch", "return", "function", "else", "do", "try", "with"}


def is_code_file(path):
    normalized = path.replace(os.sep, "/")
    return normalized.endswith(CODE_EXTENSIONS) and not any(f"/{d}" in f"/{normalized}" for d in SKIPPED_DIRS)


def strip_markers(text):
    return re.sub(r"^\{?/\*+!?|\*+/\}?$|^//+|^\*+", "", text.strip()).strip()


def is_directive(text):
    body = strip_markers(text)
    return not body or bool(DIRECTIVE.match(body))


def is_declaration(line, is_go):
    if is_go:
        return bool(re.match(r"^(func|type)\b", line))
    stripped = line.strip()
    if stripped.startswith("@"):
        return True
    if not line[:1].isspace():
        return stripped.startswith("declare ") or bool(TS_TOP_DECL.match(TS_MODIFIERS.sub("", stripped)))
    method = TS_METHOD.match(line)
    looks_like_call = "=>" in line or re.search(r"\(\s*[\"'`]", line)
    if method and method.group(4) not in CONTROL_KEYWORDS and not looks_like_call:
        return True
    return bool(TS_ARROW_PROPERTY.match(line) or TS_NESTED_FUNCTION.match(line))


def trailing_comment(line):
    code = STRING_LITERAL.sub('""', line)
    for marker in ("//", "/*"):
        index = code.find(marker)
        while index != -1:
            before = code[:index]
            jsx_comment = marker == "/*" and before.endswith("{")
            if before.strip() and not before.endswith(":") and not jsx_comment:
                comment = code[index:]
                return None if is_directive(comment) else comment.strip()
            index = code.find(marker, index + 2)
    return None


def find_violations(text, path):
    is_go = path.endswith(".go")
    lines = text.splitlines()
    violations = []
    group = []
    in_block = False

    def close_group(next_line):
        meaningful = [(n, t) for n, t in group if not is_directive(t)]
        if not meaningful:
            if not any(strip_markers(t) for _, t in group):
                violations.extend(group)
            return
        body = " ".join(strip_markers(t) for _, t in meaningful)
        sentences = len(re.findall(r"[.!?](?=\s|$)", body))
        is_doc = (
            next_line is not None
            and is_declaration(next_line, is_go)
            and len(meaningful) <= MAX_DOC_LINES
            and sentences <= MAX_DOC_SENTENCES
        )
        if not is_doc:
            violations.extend(meaningful)

    for number, line in enumerate(lines, 1):
        stripped = line.strip()
        if in_block:
            group.append((number, stripped))
            if "*/" in stripped:
                in_block = False
            continue
        if stripped.startswith(("//", "/*", "{/*")):
            if stripped.startswith(("/*", "{/*")) and "*/" not in stripped:
                in_block = True
            group.append((number, stripped))
            continue
        if group:
            close_group(line if stripped else None)
            group = []
        trailing = trailing_comment(line) if stripped and not stripped.startswith("#!") else None
        if trailing:
            violations.append((number, trailing))
    if group:
        close_group(None)
    return violations


def new_violations(old_text, new_text, path):
    if not is_code_file(path):
        return []
    remaining = Counter(t for _, t in find_violations(old_text, path))
    reported = []
    for number, comment in find_violations(new_text, path):
        if remaining[comment] > 0:
            remaining[comment] -= 1
        else:
            reported.append((path, number, comment))
    return reported


def read_file(path):
    try:
        with open(path, encoding="utf-8") as handle:
            return handle.read()
    except (OSError, UnicodeDecodeError):
        return ""


def apply_edit(text, old, new, replace_all):
    if not old:
        return new
    return text.replace(old, new) if replace_all else text.replace(old, new, 1)


def proposed_content(tool_name, tool_input):
    path = tool_input.get("file_path", "")
    current = read_file(path)
    if tool_name == "Write":
        return current, tool_input.get("content", "")
    if tool_name == "Edit":
        edits = [tool_input]
    else:
        edits = tool_input.get("edits", [])
    updated = current
    for edit in edits:
        updated = apply_edit(updated, edit.get("old_string", ""), edit.get("new_string", ""), edit.get("replace_all", False))
    return current, updated


def describe(violations, root=None):
    shown = []
    for path, number, comment in violations[:12]:
        display = os.path.relpath(path, root) if root and os.path.isabs(path) else path
        shown.append(f"  {display}:{number}  {comment[:100]}")
    more = f"\n  ...and {len(violations) - 12} more" if len(violations) > 12 else ""
    return (
        "Comment policy: do not add comments inside functions, between struct/interface/object members, "
        "or at the end of code lines. Code must explain itself through naming and small functions. "
        f"The only allowed comment is a doc comment of one or two short sentences (max {MAX_DOC_LINES} lines) "
        "directly above a function, method, or top-level type declaration. Tool directives "
        "(go:, nolint, eslint-, @ts-) are fine. Remove these comments:\n" + "\n".join(shown) + more
    )


def git(*args):
    return subprocess.run(["git", *args], capture_output=True, text=True, check=False).stdout


def uncommitted_violations():
    root = git("rev-parse", "--show-toplevel").strip()
    if not root:
        return root, []
    os.chdir(root)
    changed = set(git("diff", "--name-only", "HEAD").split()) | set(git("ls-files", "--others", "--exclude-standard").split())
    violations = []
    for path in sorted(p for p in changed if is_code_file(p) and os.path.isfile(p)):
        violations += new_violations(git("show", f"HEAD:{path}"), read_file(path), path)
    return root, violations


def run_pretool(payload):
    tool_input = payload.get("tool_input", {})
    path = tool_input.get("file_path", "")
    if not is_code_file(path):
        return
    old_text, new_text = proposed_content(payload.get("tool_name", ""), tool_input)
    violations = new_violations(old_text, new_text, path)
    if violations:
        print(json.dumps({
            "hookSpecificOutput": {
                "hookEventName": "PreToolUse",
                "permissionDecision": "deny",
                "permissionDecisionReason": describe(violations, payload.get("cwd")),
            }
        }))


def run_stop(payload):
    if payload.get("stop_hook_active"):
        return
    root, violations = uncommitted_violations()
    if violations:
        print(json.dumps({"decision": "block", "reason": describe(violations, root)}))


def run_check():
    _, violations = uncommitted_violations()
    if violations:
        print(describe(violations))
        sys.exit(1)
    print("No new comments outside the policy.")


def run_audit(paths):
    show_lines = "--lines" in paths
    paths = [p for p in paths if p != "--lines"]
    root = git("rev-parse", "--show-toplevel").strip()
    os.chdir(root)
    tracked = git("ls-files", *paths).split()
    violations = []
    for path in (p for p in tracked if is_code_file(p) and os.path.isfile(p)):
        violations += [(path, n, t) for n, t in find_violations(read_file(path), path)]
    if show_lines:
        for path, number, comment in violations:
            print(f"{path}:{number}  {comment[:140]}")
    per_file = Counter(path for path, _, _ in violations)
    for path, count in per_file.most_common():
        print(f"{count:6d}  {path}")
    print(f"{len(violations)} comments outside the policy in {len(per_file)} files")
    sys.exit(1 if violations else 0)


def main():
    mode = sys.argv[1] if len(sys.argv) > 1 else "check"
    if mode == "check":
        run_check()
        return
    if mode == "audit":
        run_audit(sys.argv[2:])
        return
    try:
        payload = json.load(sys.stdin)
    except (json.JSONDecodeError, ValueError):
        return
    if mode == "pretool":
        run_pretool(payload)
    elif mode == "stop":
        run_stop(payload)


if __name__ == "__main__":
    main()
