package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mlahr/gommit/internal/config"
	"github.com/mlahr/gommit/internal/git"
	"github.com/mlahr/gommit/internal/llm"
	"github.com/mlahr/gommit/internal/prompt"
	"github.com/mlahr/gommit/internal/ui"
)

var version = "dev"

const (
	scopeStaged           = "staged only"
	scopeStagedUnstaged   = "staged + unstaged"
	scopeAllWithUntracked = "staged + unstaged + untracked"
)

func main() {
	var includeUnstaged bool
	var includeAll bool
	var autoAccept bool
	var dumpContext bool
	var showVersion bool
	var maxPromptCharsFlag int
	var providerFlag string
	var modelFlag string
	var baseURLFlag string
	var diffBaseFlag string
	var styleFlag string
	var configPathFlag string
	var tagFlag string
	var skipCI bool
	var noVerify bool
	var dryRun bool
	var ignoreEmpty bool
	var openRouterRefFlag string
	var openRouterTitleFlag string
	var noteCommandFlag string
	var noteRefFlag string

	flag.Usage = func() {
		out := flag.CommandLine.Output()
		cfgDefaults := config.DefaultConfig()
		cfgPath, err := config.DefaultConfigPath()
		if err != nil {
			cfgPath = "~/.config/gommit/config.toml"
		}

		fmt.Fprintln(out, "Usage: gommit [options]")
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "Options:")
		fmt.Fprintln(out, "  --version                show version and exit")
		fmt.Fprintln(out, "  -u, --include-unstaged   include staged + unstaged")
		fmt.Fprintln(out, "  -A, --include-all        include staged + unstaged + untracked")
		fmt.Fprintln(out, "  -f, --accept             auto-accept proposed result")
		fmt.Fprintln(out, "  -n, --dry-run            generate and print commit message only")
		fmt.Fprintln(out, "  -I, --ignore-empty       exit 0 if no changes found")
		fmt.Fprintln(out, "  -d, --dump-context       print LLM request JSON and exit")
		fmt.Fprintln(out, "      --max-prompt-chars   max chars for user prompt (0 = no limit)")
		fmt.Fprintf(out, "  -p, --provider string    llm provider (openai, openrouter, anthropic) (default: %s)\n", cfgDefaults.Provider)
		fmt.Fprintln(out, "  -m, --model string       model name (required unless set in config/env)")
		fmt.Fprintln(out, "  -b, --base-url string    base url for openai-compatible api")
		fmt.Fprintf(out, "                           default: %s (openai), %s (openrouter)\n", config.DefaultBaseURL("openai"), config.DefaultBaseURL("openrouter"))
		fmt.Fprintln(out, "      --diff-base string   generate message from changes since rev/ref")
		fmt.Fprintln(out, "  -t, --tag string         append [STRING] to commit message")
		fmt.Fprintln(out, "  -s, --skip-ci            shortcut for --tag \"skip ci\"")
		fmt.Fprintln(out, "      --no-verify          pass --no-verify to git commit")
		fmt.Fprintln(out, "      --note-command string  shell command whose stdout is attached as a git note after commit")
		fmt.Fprintln(out, "      --note-ref string    git notes ref for --note-command output (default: refs/notes/commits)")
		fmt.Fprintf(out, "      --style string       commit style (conventional or freeform) (default: %s)\n", cfgDefaults.Style)
		fmt.Fprintf(out, "  -c, --config string      path to config file (default: %s)\n", cfgPath)
		fmt.Fprintln(out, "  -r, --openrouter-referer string  openrouter HTTP-Referer header")
		fmt.Fprintln(out, "  -T, --openrouter-title string    openrouter X-Title header")
	}

	flag.BoolVar(&includeUnstaged, "u", false, "include staged + unstaged")
	flag.BoolVar(&includeUnstaged, "include-unstaged", false, "include staged + unstaged")
	flag.BoolVar(&includeAll, "A", false, "include staged + unstaged + untracked")
	flag.BoolVar(&includeAll, "include-all", false, "include staged + unstaged + untracked")
	flag.BoolVar(&autoAccept, "f", false, "auto-accept proposed result")
	flag.BoolVar(&autoAccept, "accept", false, "auto-accept proposed result")
	flag.BoolVar(&dryRun, "n", false, "generate and print commit message only")
	flag.BoolVar(&dryRun, "dry-run", false, "generate and print commit message only")
	flag.BoolVar(&ignoreEmpty, "I", false, "exit 0 if no changes found")
	flag.BoolVar(&ignoreEmpty, "ignore-empty", false, "exit 0 if no changes found")
	flag.BoolVar(&dumpContext, "d", false, "print LLM request JSON and exit")
	flag.BoolVar(&dumpContext, "dump-context", false, "print LLM request JSON and exit")
	flag.BoolVar(&showVersion, "version", false, "show version and exit")
	flag.IntVar(&maxPromptCharsFlag, "max-prompt-chars", -1, "max chars for user prompt (0 = no limit)")
	flag.StringVar(&providerFlag, "p", "", "llm provider (openai, openrouter, anthropic)")
	flag.StringVar(&providerFlag, "provider", "", "llm provider (openai, openrouter, anthropic)")
	flag.StringVar(&modelFlag, "m", "", "model name")
	flag.StringVar(&modelFlag, "model", "", "model name")
	flag.StringVar(&baseURLFlag, "b", "", "base url for openai-compatible api")
	flag.StringVar(&baseURLFlag, "base-url", "", "base url for openai-compatible api")
	flag.StringVar(&diffBaseFlag, "diff-base", "", "generate message from changes since rev/ref")
	flag.StringVar(&tagFlag, "t", "", "append [STRING] to commit message")
	flag.StringVar(&tagFlag, "tag", "", "append [STRING] to commit message")
	flag.BoolVar(&skipCI, "s", false, "shortcut for --tag \"skip ci\"")
	flag.BoolVar(&skipCI, "skip-ci", false, "shortcut for --tag \"skip ci\"")
	flag.BoolVar(&noVerify, "no-verify", false, "pass --no-verify to git commit")
	flag.StringVar(&noteCommandFlag, "note-command", "", "shell command whose stdout is attached as a git note after commit")
	flag.StringVar(&noteRefFlag, "note-ref", "refs/notes/commits", "git notes ref for --note-command output")
	flag.StringVar(&styleFlag, "style", "", "commit style (conventional or freeform)")
	flag.StringVar(&configPathFlag, "c", "", "path to config file")
	flag.StringVar(&configPathFlag, "config", "", "path to config file")
	flag.StringVar(&openRouterRefFlag, "r", "", "openrouter HTTP-Referer header")
	flag.StringVar(&openRouterRefFlag, "openrouter-referer", "", "openrouter HTTP-Referer header")
	flag.StringVar(&openRouterTitleFlag, "T", "", "openrouter X-Title header")
	flag.StringVar(&openRouterTitleFlag, "openrouter-title", "", "openrouter X-Title header")
	flag.Parse()
	noteCommandProvided := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "note-command" {
			noteCommandProvided = true
		}
	})

	if showVersion {
		fmt.Println("gommit", version)
		return
	}

	if dryRun {
		autoAccept = true
	}

	if skipCI {
		if tagFlag != "" {
			fatal("--tag and --skip-ci cannot be used together")
		}
		tagFlag = "skip ci"
	}
	noteOptions := NoteOptions{
		Command: strings.TrimSpace(noteCommandFlag),
		Ref:     strings.TrimSpace(noteRefFlag),
		Base:    strings.TrimSpace(diffBaseFlag),
	}
	if noteCommandProvided && noteOptions.Command == "" {
		fatal("--note-command cannot be empty")
	}
	if noteOptions.Command != "" && noteOptions.Ref == "" {
		fatal("--note-ref cannot be empty when --note-command is set")
	}

	cfgPath := configPathFlag
	if cfgPath == "" {
		var err error
		cfgPath, err = config.DefaultConfigPath()
		if err != nil {
			fatal(err.Error())
		}
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		fatal(err.Error())
	}
	config.ApplyEnvOverrides(&cfg)

	if providerFlag != "" {
		cfg.Provider = providerFlag
	}
	if modelFlag != "" {
		cfg.Model = modelFlag
	}
	if baseURLFlag != "" {
		cfg.BaseURL = baseURLFlag
	}
	if styleFlag != "" {
		cfg.Style = styleFlag
	}
	if maxPromptCharsFlag >= 0 {
		cfg.MaxPromptChars = maxPromptCharsFlag
	}
	if openRouterRefFlag != "" {
		cfg.OpenRouterRef = openRouterRefFlag
	}
	if openRouterTitleFlag != "" {
		cfg.OpenRouterTitle = openRouterTitleFlag
	}

	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if provider == "" {
		provider = "openai"
	}

	if cfg.BaseURL == "" {
		cfg.BaseURL = config.DefaultBaseURL(provider)
	}
	if provider == "anthropic" && cfg.BaseURL == "" {
		fatal("anthropic requires an OpenAI-compatible base URL; set --base-url or config base_url")
	}
	if cfg.Model == "" {
		fatal("model is required; set --model or config model")
	}

	apiKey, err := config.ResolveAPIKey(provider, cfg.APIKeyEnv)
	if err != nil {
		fatal(err.Error())
	}

	root, err := git.RepoRoot()
	if err != nil {
		fatal(err.Error())
	}

	scope := git.ScopeStaged
	scopeLabel := scopeStaged
	if includeAll {
		scope = git.ScopeAll
		scopeLabel = scopeAllWithUntracked
	} else if includeUnstaged {
		scope = git.ScopeStagedUnstaged
		scopeLabel = scopeStagedUnstaged
	}

	spinnerOut := io.Writer(os.Stderr)
	if dryRun {
		spinnerOut = io.Discard
	}

	diffSpinner := ui.StartSpinner(spinnerOut, "Collecting diff")
	var result git.DiffResult
	if strings.TrimSpace(diffBaseFlag) != "" {
		diffBaseFlag = strings.TrimSpace(diffBaseFlag)
		scopeLabel = scopeLabel + " changes since " + diffBaseFlag
		result, err = git.CollectDiffFromBase(root, diffBaseFlag, scope, cfg.PerFileLimit)
	} else {
		result, err = git.CollectDiff(root, scope, cfg.PerFileLimit)
	}
	diffSpinner.Stop()
	if err != nil {
		fatal(err.Error())
	}
	if strings.TrimSpace(result.Diff) == "" && len(result.Binary) == 0 {
		if ignoreEmpty {
			return
		}
		fatal("no changes found for selected diff scope")
	}
	changedFiles := changedFilesFromResult(result)

	headers := map[string]string{}
	if provider == "openrouter" {
		if cfg.OpenRouterRef != "" {
			headers["HTTP-Referer"] = cfg.OpenRouterRef
		}
		if cfg.OpenRouterTitle != "" {
			headers["X-Title"] = cfg.OpenRouterTitle
		}
	}
	client := llm.NewClient(cfg.BaseURL, apiKey, cfg.Model, headers, cfg.Timeout)
	ctx := context.Background()

	var refinementHint string
	for {
		singlePrompt := prompt.BuildSinglePromptWithMax(cfg.Style, scopeLabel, result.Diff, result.Binary, result.TruncatedFiles, cfg.MaxPromptChars)

		// Append refinement hint if provided
		if refinementHint != "" {
			singlePrompt += "\n\nAdditional guidance: " + refinementHint
		}

		if dumpContext {
			dumpLLMContext(client, prompt.SystemPrompt(), singlePrompt)
			return
		}
		msgSpinner := ui.StartSpinner(spinnerOut, "Generating commit message")
		message, err := client.ChatCompletion(ctx, prompt.SystemPrompt(), singlePrompt)
		msgSpinner.Stop()
		if err != nil {
			fatal(err.Error())
		}
		if cfg.CleanOutput {
			cleaned := prompt.CleanResponse(message)
			logClean(message, cleaned)
			message = cleaned
		}

		// Clear refinement hint after use
		refinementHint = ""
		message = appendTag(message, tagFlag)

		if dryRun {
			fmt.Print(message)
			return
		}

		fmt.Println("Proposed commit message:")
		fmt.Println("---")
		fmt.Println(message)
		fmt.Println("---")

		ui.DisplayFileBox(os.Stdout, changedFiles, 5)
		fmt.Println()

		if autoAccept {
			if strings.TrimSpace(message) == "" {
				fatal("empty commit message")
			}
			commit, err := commitMessage(root, message, scope, noVerify)
			if err != nil {
				fatal(err.Error())
			}
			if err := attachNote(root, commit, noteOptions); err != nil {
				fatal(err.Error())
			}
			fmt.Println("Commit created.")
			return
		}

		action, err := ui.SelectOption(
			"What would you like to do with this commit message?",
			[]string{"Accept", "Edit in editor", "Retry generation", "Cancel"},
		)
		if err != nil {
			fatal(err.Error())
		}
		switch action {
		case "Cancel":
			return
		case "Retry generation":
			hint, err := ui.PromptInput(
				"Refinement hint (optional, press Enter to skip)",
				"e.g., make it shorter, focus on bug fix, etc.",
			)
			if err != nil {
				fatal(err.Error())
			}
			refinementHint = strings.TrimSpace(hint)
			continue
		case "Edit in editor":
			message, err = ui.EditInEditor(message)
			if err != nil {
				fatal(err.Error())
			}
			if strings.TrimSpace(message) == "" {
				fatal("empty commit message after edit")
			}
			commit, err := commitMessage(root, message, scope, noVerify)
			if err != nil {
				fatal(err.Error())
			}
			if err := attachNote(root, commit, noteOptions); err != nil {
				fatal(err.Error())
			}
			fmt.Println("Commit created.")
			return
		case "Accept":
			if strings.TrimSpace(message) == "" {
				fatal("empty commit message")
			}
			commit, err := commitMessage(root, message, scope, noVerify)
			if err != nil {
				fatal(err.Error())
			}
			if err := attachNote(root, commit, noteOptions); err != nil {
				fatal(err.Error())
			}
			fmt.Println("Commit created.")
			return
		}
	}
}

func appendTag(message, tag string) string {
	if tag == "" {
		return message
	}
	subject, body, hasBody := strings.Cut(strings.TrimRight(message, "\n"), "\n")
	subject = strings.TrimRight(subject, " ") + " [" + tag + "]"
	if hasBody {
		return subject + "\n" + body
	}
	return subject
}

type NoteOptions struct {
	Command string
	Ref     string
	Base    string
}

func commitMessage(root, message string, scope git.DiffScope, noVerify bool) (string, error) {
	file, err := os.CreateTemp("", "gommit-commit-*.txt")
	if err != nil {
		return "", err
	}
	defer os.Remove(file.Name())
	if _, err := file.WriteString(strings.TrimSpace(message) + "\n"); err != nil {
		_ = file.Close()
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}

	if scope == git.ScopeAll {
		if err := runGitCmd(root, "add", "."); err != nil {
			return "", err
		}
	}

	args := buildCommitArgs(scope, filepath.Clean(file.Name()), noVerify)
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return gitOutput(root, "rev-parse", "HEAD")
}

func buildCommitArgs(scope git.DiffScope, messageFile string, noVerify bool) []string {
	args := []string{"commit"}
	switch scope {
	case git.ScopeStagedUnstaged, git.ScopeAll:
		args = append(args, "-a")
	}
	if noVerify {
		args = append(args, "--no-verify")
	}
	args = append(args, "-F", messageFile)
	return args
}

func runGitCmd(root string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func attachNote(root, commit string, opts NoteOptions) error {
	if opts.Command == "" {
		return nil
	}
	commit = strings.TrimSpace(commit)
	if commit == "" {
		return fmt.Errorf("cannot attach note: created commit is empty")
	}

	note, err := runNoteCommand(root, commit, opts)
	if err != nil {
		return fmt.Errorf("commit %s created, but note command failed: %w", commit, err)
	}
	if err := addGitNote(root, commit, opts.Ref, note); err != nil {
		return fmt.Errorf("commit %s created, but attaching git note failed: %w", commit, err)
	}
	return nil
}

func runNoteCommand(root, commit string, opts NoteOptions) ([]byte, error) {
	command := expandNoteCommand(opts.Command, opts.Base, commit)
	cmd := shellCommand(command)
	cmd.Dir = root

	if noteCommandUsesStdin(opts.Command) {
		diff, err := selectedCommitDiff(root, opts.Base, commit)
		if err != nil {
			return nil, err
		}
		cmd.Stdin = strings.NewReader(diff)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("%s", msg)
	}
	return stdout.Bytes(), nil
}

func addGitNote(root, commit, noteRef string, note []byte) error {
	file, err := os.CreateTemp("", "gommit-note-*.txt")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(note); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}

	return runGitQuiet(root, "notes", "--ref", noteRef, "add", "-f", "--allow-empty", "-F", filepath.Clean(file.Name()), commit)
}

func expandNoteCommand(command, base, commit string) string {
	command = strings.ReplaceAll(command, "{base}", shellQuote(base))
	return strings.ReplaceAll(command, "{commit}", shellQuote(commit))
}

func noteCommandUsesStdin(command string) bool {
	return !strings.Contains(command, "{base}") && !strings.Contains(command, "{commit}")
}

func selectedCommitDiff(root, base, commit string) (string, error) {
	base = strings.TrimSpace(base)
	commit = strings.TrimSpace(commit)
	if base == "" {
		var err error
		base, err = firstParentOrEmptyTree(root, commit)
		if err != nil {
			return "", err
		}
	}
	return gitOutput(root, "diff", base, commit)
}

func firstParentOrEmptyTree(root, commit string) (string, error) {
	parent, err := gitOutput(root, "rev-parse", commit+"^")
	if err == nil {
		return parent, nil
	}
	return gitOutput(root, "hash-object", "-t", "tree", "/dev/null")
}

func gitOutput(root string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			msg := strings.TrimSpace(string(exitErr.Stderr))
			if msg != "" {
				return "", fmt.Errorf("git %s failed: %s", strings.Join(args, " "), msg)
			}
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func runGitQuiet(root string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("git %s failed: %s", strings.Join(args, " "), msg)
	}
	return nil
}

func shellCommand(command string) *exec.Cmd {
	shell := strings.TrimSpace(os.Getenv("SHELL"))
	if shell == "" {
		shell = "/bin/sh"
	}
	return exec.Command(shell, "-c", command)
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func changedFilesFromResult(result git.DiffResult) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, chunk := range git.SplitDiffChunks(result.Diff) {
		path := strings.TrimSpace(git.ParseDiffPath(chunk))
		if path == "" {
			continue
		}
		path = filepath.Clean(path)
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		out = append(out, path)
	}
	for _, bf := range result.Binary {
		path := strings.TrimSpace(bf.Path)
		if path == "" {
			continue
		}
		path = filepath.Clean(path)
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		out = append(out, path)
	}
	return out
}

func dumpLLMContext(client *llm.Client, systemPrompt, userPrompt string) {
	payload, err := client.BuildChatPayload(systemPrompt, userPrompt)
	if err != nil {
		fatal(err.Error())
	}
	fmt.Println(string(payload))
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "gommit:", msg)
	os.Exit(1)
}

func logClean(raw, cleaned string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(home, ".gommit.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "--- %s\nraw:\n%s\ncleaned:\n%s\n", time.Now().Format(time.RFC3339), raw, cleaned)
}
