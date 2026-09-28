# gommit

Generate git commit messages using an OpenAI-compatible LLM.

## Install

Debian-based Linux from GitHub Releases:

```bash
curl -fsSL https://raw.githubusercontent.com/mlahr/gommit/main/install.sh | bash
```

Manual install:

1. Download the `gommit_*_linux_<arch>.tar.gz` or `gommit_*_linux_<arch>.deb` asset from the release page, where `<arch>` is `amd64` or `arm64`.
2. For tar.gz:

```bash
tar -xzf gommit_*_linux_<arch>.tar.gz
sudo install -m 0755 gommit /usr/local/bin/gommit
```

3. For .deb:

```bash
sudo apt-get install ./gommit_*_linux_<arch>.deb
```

Build from source:

```bash
go build -o gommit
```

Install with Go:

```bash
go install github.com/mlahr/gommit@latest
```

## Usage

```bash
# staged only (default)
./gommit --provider openai --model gpt-4o-mini

# staged + unstaged
./gommit -u --provider openai --model gpt-4o-mini

# staged + unstaged + untracked
./gommit -A --provider openai --model gpt-4o-mini

# generate a message from changes since a previous checkpoint
./gommit -A -n --diff-base previous-autosnap-checkpoint --provider openai --model gpt-4o-mini

# attach tool output as a git note on the created commit
./gommit -A --diff-base previous-autosnap-checkpoint \
  --note-command 'diffcog --delta-totals {base} {commit}' \
  --provider openai --model gpt-4o-mini
```

## Flags

- `-u`, `--include-unstaged`: include staged + unstaged
- `-A`, `--include-all`: include staged + unstaged + untracked
- `--diff-base`: generate the commit message from changes since a rev/ref
- `-t`, `--tag`: append `[STRING]` to the commit message
- `-s`, `--skip-ci`: shortcut for `--tag "skip ci"`
- `-f`, `--accept`: auto-accept proposed result (skips prompt)
- `--note-command`: shell command whose stdout is attached as a git note after commit
- `--note-ref`: git notes ref for `--note-command` output (default: `refs/notes/commits`)
- `-d`, `--dump-context`: print LLM request JSON and exit
- `--max-prompt-chars`: max chars for user prompt (0 = no limit)
- `-p`, `--provider`: `openai`, `openrouter`, `anthropic`
- `-m`, `--model`: model name (required unless set in config)
- `-b`, `--base-url`: OpenAI-compatible base URL
- `-t`, `--style`: `conventional` or `freeform`
- `-c`, `--config`: config file path
- `-r`, `--openrouter-referer`: set OpenRouter `HTTP-Referer` header
- `-T`, `--openrouter-title`: set OpenRouter `X-Title` header

## Config

Default config path: `~/.config/gommit/config.toml`

Example:

```toml
provider = "openai"
model = "gpt-4o-mini"
base_url = "https://api.openai.com/v1"
style = "conventional"
per_file_limit = 20000
max_prompt_chars = 0
clean_output = true
openrouter_referer = "https://example.com"
openrouter_title = "gommit"
api_key_env = "MY_OPENAI_KEY"
```

When `clean_output = true`, gommit strips common LLM preamble and postamble
(e.g., "Here is the commit message:", trailing explanations) from the response.
Useful with cheaper models that tend to wrap the commit message in conversational text.

## Environment Variables

API keys:

- `OPENAI_API_KEY`
- `OPENROUTER_API_KEY`
- `ANTHROPIC_API_KEY`
- `GOMMIT_API_KEY` (fallback)

When `api_key_env` is set (config) or `GOMMIT_API_KEY_ENV` is set, only that
variable is read; the provider-specific vars and `GOMMIT_API_KEY` are ignored.

Config overrides:

- `GOMMIT_PROVIDER`
- `GOMMIT_MODEL`
- `GOMMIT_BASE_URL`
- `GOMMIT_STYLE`
- `GOMMIT_PER_FILE_LIMIT`
- `GOMMIT_MAX_PROMPT_CHARS`
- `GOMMIT_CLEAN_OUTPUT`
- `GOMMIT_OPENROUTER_REFERER`
- `GOMMIT_OPENROUTER_TITLE`
- `GOMMIT_API_KEY_ENV`
- `OPENROUTER_REFERER`
- `OPENROUTER_TITLE`

## Git Notes

`--note-command` runs after a commit is created. Its stdout is written to a
git note on the new commit, overwriting any existing note in the selected notes
ref.

The command is evaluated by your shell. Use `{base}` and `{commit}` placeholders
to receive the `--diff-base` value and the created commit SHA:

```bash
gommit -A --diff-base previous-autosnap-checkpoint \
  --note-command 'diffcog --delta-totals {base} {commit}'
```

If the command contains neither placeholder, `gommit` pipes the selected
post-commit diff to stdin. With `--diff-base`, that diff is `git diff
<diff-base> <created-commit>`. Without `--diff-base`, it is the created commit's
first-parent diff.

## Release (Linux amd64/arm64 + .deb)

Releases are built by GitHub Actions using GoReleaser on tag pushes.

Release steps:

1. Create a version tag and push it:

```bash
git tag v0.1.0
git push origin v0.1.0
```

2. Wait for the `release` workflow to finish.
3. Download assets from the GitHub release page:

- `gommit_*_linux_amd64.tar.gz`
- `gommit_*_linux_amd64.deb`
- `gommit_*_linux_arm64.tar.gz`
- `gommit_*_linux_arm64.deb`
- `checksums.txt`

Local dry run (optional):

```bash
goreleaser release --snapshot --clean
```

## License

MIT
