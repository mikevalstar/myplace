package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/mikevalstar/myplace/internal/drift"
	"github.com/mikevalstar/myplace/internal/version"
)

// Annotation keys carry the machine-facing facts cobra doesn't model itself.
// They live in each command's cobra Annotations, set at the definition site,
// and are read back by describe() to build the agent manifest. help_test.go
// walks the tree and fails if a command is missing the ones it owes, so the
// manifest can't silently drift from the code.
const (
	annHeadless     = "myplace.headless"      // canonical headless invocation, e.g. "myplace status --json"
	annRequired     = "myplace.required"      // comma-separated flag names required off a TTY
	annExitCodes    = "myplace.exit_codes"    // "code=meaning" pairs joined by ";"
	annOutputSchema = "myplace.output_schema" // doc path describing the --json output shape
	annInteractive  = "myplace.interactive"   // "true" when the command has a human-facing interactive path
	annNote         = "myplace.note"          // optional one-line, agent-facing tip
)

// Shared exit-code descriptions, so commands with the same contract stay
// identical. Format: "code=meaning" pairs joined by ";".
const (
	exitCodesDrift    = "0=in sync;1=drifted;2=unknown (e.g. offline);3=error"
	exitCodesConverge = "0=success;1=completed with per-item failures;3=error, or a needed decision was not supplied non-interactively"
	exitCodesOutdated = "0=all current;1=outdated packages available;3=error (no source could be queried)"
	exitCodesSysinfo  = "0=success;3=error (fastfetch unavailable or failed)"

	// The canonical, tool-wide table (the union across commands) advertised at
	// the top of the manifest and brief; matches the headless CLI spec.
	globalExitCodes = "0=in sync / success;1=drifted / completed with per-item failures;2=unknown (e.g. offline);3=error, or a needed decision was not supplied non-interactively"
)

// manifest is the `myplace help --json` document: the whole command surface in
// one schema-versioned blob an agent can consume in a single read.
type manifest struct {
	Schema    int               `json:"schema"`
	Tool      string            `json:"tool"`
	Version   string            `json:"version"`
	Summary   string            `json:"summary"`
	ExitCodes map[string]string `json:"exit_codes"`
	Commands  []commandDesc     `json:"commands"`
}

type commandDesc struct {
	Name                string            `json:"name"`
	Summary             string            `json:"summary"`
	Headless            string            `json:"headless,omitempty"`
	Interactive         bool              `json:"interactive"`
	RequiredForHeadless []string          `json:"required_for_headless,omitempty"`
	Flags               []flagDesc        `json:"flags,omitempty"`
	ExitCodes           map[string]string `json:"exit_codes,omitempty"`
	OutputSchema        string            `json:"output_schema,omitempty"`
	Note                string            `json:"note,omitempty"`
}

type flagDesc struct {
	Name                string `json:"name"`
	Shorthand           string `json:"shorthand,omitempty"`
	Type                string `json:"type"`
	Default             string `json:"default"`
	Description         string `json:"description"`
	RequiredForHeadless bool   `json:"required_for_headless,omitempty"`
}

// newHelpCmd is installed via root.SetHelpCommand. Without a format flag it
// behaves like cobra's normal help; --json and --llm render the agent views
// from the same command tree. A trailing command name narrows either view.
func newHelpCmd() *cobra.Command {
	var jsonOut, llmOut bool
	cmd := &cobra.Command{
		Use:   "help [command]",
		Short: "Help about any command (add --json or --llm for the agent manifest/brief)",
		Long: "Help about any command.\n\n" +
			"--json emits a machine-readable manifest of every command (flags, defaults,\n" +
			"which flags are required off a TTY, exit codes, output-schema docs).\n" +
			"--llm emits a copy-paste brief with workflow recipes, sized for an LLM context.\n" +
			"Add a command name to scope either view to one command.",
		Annotations: map[string]string{
			annHeadless:     "myplace help --llm",
			annExitCodes:    "0=success",
			annOutputSchema: "docs/features/llm-friendly-help.md",
			annInteractive:  "true",
			annNote:         "--json emits the command manifest; --llm a copy-paste agent brief. `myplace help <command> --json|--llm` scopes either to one command.",
		},
		Run: func(cmd *cobra.Command, args []string) {
			root := cmd.Root()

			var target *cobra.Command
			if len(args) > 0 {
				c, _, err := root.Find(args)
				if err != nil || c == nil || c == root {
					fmt.Fprintf(os.Stderr, "unknown command %q for %q\n", strings.Join(args, " "), root.Name())
					os.Exit(3)
				}
				target = c
			}

			switch {
			case jsonOut:
				m := describe(root)
				if target != nil {
					m = m.only(target.Name())
				}
				emitJSON(m)
			case llmOut:
				fmt.Print(renderLLMBrief(describe(root), targetName(target)))
			case target != nil:
				_ = target.Help()
			default:
				_ = root.Help()
			}
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit the command manifest as one JSON document")
	cmd.Flags().BoolVar(&llmOut, "llm", false, "emit a copy-paste agent brief (ANSI-free, with workflow recipes)")
	return cmd
}

func targetName(c *cobra.Command) string {
	if c == nil {
		return ""
	}
	return c.Name()
}

// describe walks the command tree into a manifest. Bare `myplace` (the root's
// own no-arg behavior) is listed first, then each visible subcommand in
// declared order.
func describe(root *cobra.Command) manifest {
	m := manifest{
		Schema:    drift.Schema,
		Tool:      root.Name(),
		Version:   version.Version,
		Summary:   root.Short,
		ExitCodes: parseExitCodes(globalExitCodes),
	}
	m.Commands = append(m.Commands, describeCommand(root, root.Name()))
	// Include every visible subcommand, plus the help command itself (cobra's
	// IsAvailableCommand hides help, but an agent benefits from discovering
	// `help --json`/`--llm` here). The completion command is disabled in
	// newRootCmd, so it never appears. Dedupe by name, defensively, since some
	// cobra versions register the help command more than once.
	seen := map[string]bool{root.Name(): true}
	for _, c := range root.Commands() {
		if c.Hidden || len(c.Deprecated) > 0 || seen[c.Name()] {
			continue
		}
		seen[c.Name()] = true
		m.Commands = append(m.Commands, describeCommand(c, c.Name()))
	}
	return m
}

func describeCommand(c *cobra.Command, name string) commandDesc {
	ann := c.Annotations
	d := commandDesc{
		Name:         name,
		Summary:      c.Short,
		Headless:     ann[annHeadless],
		OutputSchema: ann[annOutputSchema],
		Note:         ann[annNote],
		Interactive:  ann[annInteractive] == "true",
		ExitCodes:    parseExitCodes(ann[annExitCodes]),
	}

	required := splitList(ann[annRequired])
	d.RequiredForHeadless = required
	reqSet := make(map[string]bool, len(required))
	for _, r := range required {
		reqSet[r] = true
	}

	c.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if f.Name == "help" || f.Hidden {
			return
		}
		d.Flags = append(d.Flags, flagDesc{
			Name:                f.Name,
			Shorthand:           f.Shorthand,
			Type:                f.Value.Type(),
			Default:             f.DefValue,
			Description:         f.Usage,
			RequiredForHeadless: reqSet[f.Name],
		})
	})
	return d
}

// only narrows the manifest to a single command (for `help <command> --json`).
func (m manifest) only(name string) manifest {
	var cmds []commandDesc
	for _, c := range m.Commands {
		if c.Name == name {
			cmds = append(cmds, c)
		}
	}
	m.Commands = cmds
	return m
}

// renderLLMBrief produces the ANSI-free `--llm` brief. With target set, it
// emits only that command's section; otherwise the full brief: conventions
// header, per-command reference, and the workflow recipes.
func renderLLMBrief(m manifest, target string) string {
	var b strings.Builder
	if target == "" {
		fmt.Fprintf(&b, "# %s (%s)\n%s\n\n", m.Tool, m.Version, m.Summary)
		b.WriteString("Conventions\n")
		b.WriteString("  Exit codes: " + formatExits(m.ExitCodes) + ".\n")
		b.WriteString("  All data commands accept --json: exactly one document on stdout, logs/progress on stderr.\n")
		b.WriteString("  Off a TTY, a command needing an unsupplied decision fails fast (exit 3) naming the flag — it never prompts.\n")
		b.WriteString("  bootstrap and update require --yes to run unattended.\n\n")
	}
	for _, c := range m.Commands {
		if target != "" && c.Name != target {
			continue
		}
		writeCommandBrief(&b, c)
	}
	if target == "" {
		b.WriteString(recipes)
		b.WriteString("\nDocs: docs/features/headless-cli-and-json-output.md (envelope, exit codes) · docs/workflows/ (end-to-end flows)\n")
	}
	return b.String()
}

func writeCommandBrief(b *strings.Builder, c commandDesc) {
	fmt.Fprintf(b, "## %s — %s\n", c.Name, c.Summary)
	if c.Headless != "" {
		fmt.Fprintf(b, "  headless: %s\n", c.Headless)
	}
	if len(c.RequiredForHeadless) > 0 {
		fmt.Fprintf(b, "  required off a TTY: %s\n", strings.Join(dashed(c.RequiredForHeadless), ", "))
	}
	if len(c.Flags) > 0 {
		fmt.Fprintf(b, "  flags: %s\n", strings.Join(flagSummaries(c.Flags), ", "))
	}
	if len(c.ExitCodes) > 0 {
		fmt.Fprintf(b, "  exits: %s\n", formatExits(c.ExitCodes))
	}
	if c.OutputSchema != "" {
		fmt.Fprintf(b, "  output: see %s\n", c.OutputSchema)
	}
	if c.Note != "" {
		fmt.Fprintf(b, "  note: %s\n", c.Note)
	}
	b.WriteString("\n")
}

// recipes are the editorial, end-to-end workflows — the part the command tree
// can't generate. The invocations themselves are the same headless forms the
// manifest emits, so they can't drift in their flags.
const recipes = `## Recipes
# Bootstrap a server unattended (installs chezmoi+mise, applies dotfiles, installs tools)
myplace bootstrap --profile server --yes

# Check one host, then sweep a fleet — branch on the exit code, not on stdout
myplace status --json | jq .verdict
for h in web1 web2 db1; do ssh "$h" myplace status --json >/dev/null && echo "$h ok" || echo "$h drifted"; done

# Update headlessly — converge-only: applies incoming + upgrades tools, never pushes local edits
myplace update --yes --json
`

func flagSummaries(flags []flagDesc) []string {
	out := make([]string, 0, len(flags))
	for _, f := range flags {
		s := "--" + f.Name
		if f.Type != "bool" {
			s += " <" + f.Type + ">"
		}
		// Surface only meaningful defaults; an empty string or `false` is noise.
		if f.Default != "" && f.Default != "false" {
			s += " (default " + f.Default + ")"
		}
		out = append(out, s)
	}
	return out
}

func dashed(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = "--" + n
	}
	return out
}

// formatExits renders an exit-code map in numeric order: "0 in sync · 1 ...".
func formatExits(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+" "+m[k])
	}
	return strings.Join(parts, " · ")
}

func parseExitCodes(s string) map[string]string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	m := make(map[string]string)
	for _, part := range strings.Split(s, ";") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			m[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return m
}

func splitList(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
