---
name: pavo
description: Use the PAVO CLI to upload a chat attachment, log in, create a conversation, and stream a design generation. Use for PAVO image-generation or chat-attachment upload requests handled by the desktop agent.
---

# PAVO

Use the bundled `pavo` CLI for PAVO requests. The supported commands are:

1. `pavo login`
2. `pavo upload`
3. `pavo conversation create`
4. `pavo stream`
5. `pavo download-result`

For design generation, use the strict sequence `pavo login` → `pavo conversation create` → `pavo stream`. Do not substitute direct `curl` calls, invent other PAVO commands, or call unrelated image/video services.

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

## Upload a chat attachment

When the user explicitly asks to upload a local PAVO chat attachment, run:

```bash
pavo upload --file "LOCAL_FILE_PATH"
```

Read `public_url` from stdout and return that URL. The CLI handles the authenticated pre-upload request and the unauthenticated object-store PUT; it never prints the temporary signed upload URL.

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

Pass the same prompt and the returned conversation ID. When the user provides local reference files, include one `--file` flag for each path; the CLI uploads them and sends the resulting public URLs as `files` in the stream request:

```bash
pavo stream \
  --conversation-id "CONVERSATION_ID" \
  --prompt "USER_PROMPT" \
  --file "LOCAL_FILE_PATH"
```

The CLI fixes `mode` to `design`. It reads the stream until `GenerationSuccess`, writes progress to stderr, and emits one final JSON object to stdout. Omit `--file` when there is no attachment; repeat it for multiple local attachments.

Use `--raw` only when diagnosing the PAVO event stream. Raw events are written to stderr so stdout remains machine-readable.

## Download a generated result

By default, return each successful result's `url`; do not download every result automatically. Download only when the user explicitly asks to download, save, or export the result, when they ask for a local path, or when an approved subsequent task requires the local image or video file.

```bash
pavo download-result \
  --url "RESULT_URL" \
  --output-path "LOCAL_OUTPUT_FILE"
```

`--output-path` must include the destination filename. The command returns `downloaded` when it saves the file and `already_exist` when it safely reuses a local file. Pass `--force` only when the user asks to replace an existing local file. If the service supplies a Unix update timestamp for the result, pass it as `--updated-at`; otherwise omit it.

Do not download pending or failed results, an empty URL, or `thumbnail_url` merely for preview. A `base64` result is not a URL download; handle it only if a dedicated local-save capability is added.

## Return results

Read `results` from the final JSON. Present each successful result's `url`, and use `thumbnail_url` only as a preview when useful. Also preserve the reported `width`, `height`, `ratio`, and `mimetype` when the user needs technical details. When a download occurs, present the returned local output path as well as the result URL.

If login, conversation creation, or streaming fails, report the CLI error and stop. Do not manufacture a result or continue with another provider.
