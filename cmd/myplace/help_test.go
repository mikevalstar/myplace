package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/mikevalstar/myplace/internal/chezmoi"
	"github.com/mikevalstar/myplace/internal/drift"
	"github.com/mikevalstar/myplace/internal/mise"
	"github.com/mikevalstar/myplace/internal/run"
)

// testRoot builds the real command tree. The clients are never exercised (the
// help views only read the tree's shape and annotations), so a no-op runner is
// fine. InitDefaultHelpCmd registers the custom help command into the tree,
// which at runtime happens during Execute.
func testRoot(t *testing.T) *cobra.Command {
	t.Helper()
	r := run.WithLogger(nil, nil)
	root := newRootCmd(r, chezmoi.New(r), mise.New(r))
	root.InitDefaultHelpCmd()
	return root
}

// visibleCommands is root + every non-hidden, non-deprecated subcommand —
// exactly the set the manifest claims to cover.
func visibleCommands(root *cobra.Command) []*cobra.Command {
	cmds := []*cobra.Command{root}
	for _, c := range root.Commands() {
		if c.Hidden || len(c.Deprecated) > 0 {
			continue
		}
		cmds = append(cmds, c)
	}
	return cmds
}

func localFlagSet(c *cobra.Command) map[string]string {
	flags := map[string]string{}
	c.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if f.Name == "help" || f.Hidden {
			return
		}
		flags[f.Name] = f.Value.Type()
	})
	return flags
}

// TestManifestCoversTree: every command in the cobra tree appears in the
// manifest exactly once, and each flag's name/type/default matches the actual
// flag definition — so the manifest is generated, never hand-maintained.
func TestManifestCoversTree(t *testing.T) {
	root := testRoot(t)
	m := describe(root)

	if m.Schema != drift.Schema {
		t.Errorf("manifest schema = %d, want %d", m.Schema, drift.Schema)
	}

	got := map[string]commandDesc{}
	for _, c := range m.Commands {
		if _, dup := got[c.Name]; dup {
			t.Errorf("command %q appears more than once in the manifest", c.Name)
		}
		got[c.Name] = c
	}

	for _, c := range visibleCommands(root) {
		cd, ok := got[c.Name()]
		if !ok {
			t.Errorf("manifest is missing command %q", c.Name())
			continue
		}
		want := localFlagSet(c)
		have := map[string]string{}
		for _, f := range cd.Flags {
			have[f.Name] = f.Type
			// Default must match the real flag's default.
			if real := c.LocalFlags().Lookup(f.Name); real != nil && real.DefValue != f.Default {
				t.Errorf("%s flag %q: manifest default %q != actual %q", c.Name(), f.Name, f.Default, real.DefValue)
			}
		}
		if len(want) != len(have) {
			t.Errorf("%s: manifest lists flags %v, command defines %v", c.Name(), have, want)
		}
		for name, typ := range want {
			if have[name] != typ {
				t.Errorf("%s flag %q: manifest type %q != actual %q", c.Name(), name, have[name], typ)
			}
		}
	}
}

// TestCommandAnnotations enforces that no command can silently go stale: each
// declares its exit codes and interactive-ness and a headless invocation;
// every command that emits --json points at an output-schema doc that exists;
// and every required_for_headless flag actually exists on the command.
func TestCommandAnnotations(t *testing.T) {
	root := testRoot(t)
	for _, c := range visibleCommands(root) {
		name := c.Name()
		ann := c.Annotations

		if parseExitCodes(ann[annExitCodes]) == nil {
			t.Errorf("%s: missing or unparseable %q annotation", name, annExitCodes)
		}
		if v := ann[annInteractive]; v != "true" && v != "false" {
			t.Errorf("%s: %q must be \"true\" or \"false\", got %q", name, annInteractive, v)
		}
		if strings.TrimSpace(ann[annHeadless]) == "" {
			t.Errorf("%s: missing %q annotation", name, annHeadless)
		}

		// A --json-emitting command owes a pointer to its output shape, and
		// that doc must exist on disk (repo root is two levels up from here).
		if c.LocalFlags().Lookup("json") != nil {
			schema := ann[annOutputSchema]
			if schema == "" {
				t.Errorf("%s: emits --json but has no %q annotation", name, annOutputSchema)
			} else if _, err := os.Stat(filepath.Join("..", "..", schema)); err != nil {
				t.Errorf("%s: output_schema %q does not exist: %v", name, schema, err)
			}
		}

		// required_for_headless must name real flags (catches rename drift).
		for _, fl := range splitList(ann[annRequired]) {
			if c.LocalFlags().Lookup(fl) == nil {
				t.Errorf("%s: required_for_headless names %q but no such flag exists", name, fl)
			}
		}
	}
}

// TestRequiredFlagsAreMarked: a flag listed in required_for_headless is flagged
// as such in the manifest, and others are not.
func TestRequiredFlagsAreMarked(t *testing.T) {
	root := testRoot(t)
	for _, c := range describe(root).Commands {
		req := map[string]bool{}
		for _, r := range c.RequiredForHeadless {
			req[r] = true
		}
		for _, f := range c.Flags {
			if f.RequiredForHeadless != req[f.Name] {
				t.Errorf("%s flag %q: required_for_headless=%v but command list says %v",
					c.Name, f.Name, f.RequiredForHeadless, req[f.Name])
			}
		}
	}
}

// TestLLMBriefIsANSIFree covers the acceptance criterion that --llm is plain
// text, and spot-checks that it teaches workflows, not just flags.
func TestLLMBriefIsANSIFree(t *testing.T) {
	root := testRoot(t)
	brief := renderLLMBrief(describe(root), "")

	if strings.ContainsRune(brief, '\x1b') {
		t.Error("--llm brief contains an ANSI escape (\\x1b)")
	}
	for _, want := range []string{
		"Conventions",
		"## status",
		"## Recipes",
		"myplace bootstrap --profile server --yes",
	} {
		if !strings.Contains(brief, want) {
			t.Errorf("--llm brief is missing %q", want)
		}
	}
}

// TestScopedBriefOmitsHeaderAndRecipes: a per-command brief is just that
// command's section.
func TestScopedBriefOmitsHeaderAndRecipes(t *testing.T) {
	root := testRoot(t)
	brief := renderLLMBrief(describe(root), "status")

	if !strings.Contains(brief, "## status") {
		t.Error("scoped brief should contain the status section")
	}
	for _, unwanted := range []string{"Conventions", "## Recipes", "## bootstrap"} {
		if strings.Contains(brief, unwanted) {
			t.Errorf("scoped brief should not contain %q", unwanted)
		}
	}
}

// TestManifestOnly narrows to a single command while preserving the envelope.
func TestManifestOnly(t *testing.T) {
	root := testRoot(t)
	m := describe(root).only("status")

	if len(m.Commands) != 1 || m.Commands[0].Name != "status" {
		t.Fatalf("only(\"status\") returned %d commands: %+v", len(m.Commands), m.Commands)
	}
	if m.Schema != drift.Schema {
		t.Errorf("only() dropped the schema (got %d)", m.Schema)
	}
}
