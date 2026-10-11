---
name: clean-comments
description: Remove comments that break the repo comment policy from a given path, without changing behavior.
disable-model-invocation: true
argument-hint: "<path, e.g. server/internal/wallet or client/src/features/lobby>"
---

# Clean comments in $ARGUMENTS

Bring every Go and TypeScript file under `$ARGUMENTS` in line with the repo comment policy. This is a comments-only change. Behavior, formatting, and test results must stay exactly the same.

## The rule

Code explains itself through names, small functions, and types. A comment is allowed only in these cases:

1. **Doc comment above a function or method.** One or two short sentences, at most three lines, saying what the function does. It sits directly on the line above the declaration with no blank line between.
2. **Doc comment above a top-level type.** Same limits, for a Go `type`, or a TypeScript `class`, `interface`, or `type`.
3. **Tool directives.** `//go:build`, `//go:embed`, `//nolint`, `// eslint-disable…`, `// @ts-expect-error`, `/* @vite-ignore */`, `// Code generated … DO NOT EDIT.` and similar.

Everything else goes:

- Comments inside function and method bodies, including test bodies.
- Comments between struct fields, interface members, object keys, switch cases, or JSX elements, including `{/* … */}`.
- Comments at the end of a code line.
- Floating comments above `var`, `const`, imports, or `describe`/`it` blocks.
- Section banners, story or ticket references (`Story 13.2`, `P18`), changelog notes, and commented-out code.
- Doc comments longer than two sentences. Shorten them to what the function does, not how or why it got that way.

## How to handle a comment that carries real knowledge

Some comments explain a non-obvious *why*, such as a security assumption, a protocol quirk, or an ordering constraint. Do not silently lose that knowledge. Pick the first option that works:

1. **Make the code say it.** Rename a variable or function, extract a small well-named helper, or introduce a named constant. For example, `const cloudflareEdgeRanges` instead of a comment above a bare slice. Keep the refactor local, mechanical, and behavior-preserving.
2. **Fold it into the doc comment** of the enclosing function, if it still fits in two sentences.
3. **If neither fits,** leave the comment in place and list it in your final report with the file and line. Do not invent a third sentence.

Never change logic, error handling, exported names, public APIs, JSON tags, SQL, or test assertions to make a comment unnecessary. If a rename would cross package boundaries or touch exported identifiers, keep the comment and report it instead.

## Process

1. Run the audit to see the scope:
   ```
   python3 .claude/hooks/comment_policy.py audit $ARGUMENTS
   ```
2. Split the file list into batches of about 15 files, grouped by directory. Hand each batch to a subagent in parallel with this whole prompt and its file list. Work directly instead if fewer than 15 files are in scope.
3. Each file: read it whole, apply the rule, re-read the result, and make sure it still reads clearly without the comments.
4. Format and verify the parts you touched:
   ```
   make lint-server test-server
   make lint-client test-client
   ```
   Run only the side that changed, and run `gofmt -w` or `npx prettier --write` on touched files first. A failure means you changed behavior, so find and revert that change.
5. Re-run the audit on `$ARGUMENTS`. The only remaining entries must be the comments you chose to keep under "If neither fits."
6. Confirm the diff only removes or shortens comments, apart from deliberate local renames. Use `git diff --stat` and skim `git diff`.

## Report

End with a short report containing:

- The number of files changed and comment lines removed.
- Each local rename or extracted helper, one line each.
- Each comment you kept, with file, line, and a one-line reason.
- The verification commands you ran and their results.

Do not commit. Leave the changes in the working tree for review.
