---
name: pavo
description: Use the PAVO CLI to log in, create a conversation, and stream a design generation. Use for PAVO image-generation requests handled by the desktop agent.
---

# PAVO

Use the bundled `pavo` CLI for PAVO requests. The supported business workflow is strictly:

1. `pavo login`
2. `pavo conversation create`
3. `pavo stream`

Do not substitute direct `curl` calls, invent other PAVO commands, download results, or call unrelated image/video services.

## Authentication

Try the requested conversation command first when a stored login may already exist. If the CLI reports that the user is not logged in, obtain the user's PAVO email and password before running login.

Prefer interactive login so the password is hidden:

```bash
pavo login --email "USER_EMAIL"
```

For a non-interactive desktop-agent terminal, use `--password` only when the user explicitly supplied the password for this login:

```bash
pavo login --email "USER_EMAIL" --password "USER_PASSWORD"
```

Never repeat the password or access token in the response, logs, summaries, or generated files. A successful login prints user information but never prints the access token.

## Create a conversation

Use the user's generation prompt unchanged:

```bash
pavo conversation create --prompt "USER_PROMPT"
```

Parse `conversation_id` from the JSON written to stdout:

```json
{"conversation_id":"338562408542949376"}
```

The CLI encodes the title in PAVO's required text-part format and fixes `folder_id` to an empty string and `kb_strict` to `false`.

## Stream the generation

Pass the same prompt and the returned conversation ID:

```bash
pavo stream \
  --conversation-id "CONVERSATION_ID" \
  --prompt "USER_PROMPT"
```

The CLI fixes `mode` to `design`. It reads the stream until `GenerationSuccess`, writes progress to stderr, and emits one final JSON object to stdout.

Use `--raw` only when diagnosing the PAVO event stream. Raw events are written to stderr so stdout remains machine-readable.

## Return results

Read `results` from the final JSON. Present each successful result's `url`, and use `thumbnail_url` only as a preview when useful. Also preserve the reported `width`, `height`, `ratio`, and `mimetype` when the user needs technical details.

If login, conversation creation, or streaming fails, report the CLI error and stop. Do not manufacture a result or continue with another provider.
