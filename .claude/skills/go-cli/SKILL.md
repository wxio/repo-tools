---
name: go-cli
description: Build Go CLIs with github.com/jpillora/opts. Use when creating a new opts-based main, adding a subcommand to an existing opts CLI, or copying/duplicating an existing subcommand into a new package. Covers struct tag conventions, command factory functions, Run() methods, and registration in builder.AddCommand chains.
---

# go-cli: building CLIs with github.com/jpillora/opts

The `github.com/jpillora/opts` library builds CLIs by reflecting on plain Go structs. Each subcommand is a struct whose fields become flags/args (via tags), with a `Run() error` method that executes it. The root command and subcommands are wired together in `init()` (or `main`) using `opts.New(...).AddCommand(...)`.

sheafdb's one opts CLI is `samples/goohm/main.go` (`goohm`): a single
root command with a `Run()` that reads standard input and writes standard
output, and a `run(in, out, errOut)` split out so `main_test.go` can drive
it. Read it first and match it; add subcommands there with the
`AddCommand` pattern below if the tool grows more than one job.

## 1. Creating a main from scratch

Minimal layout (single file, no subcommands):

```go
package main

import (
	"fmt"
	"os"

	"github.com/jpillora/opts"
)

type rootCmd struct {
	Verbose bool   `opts:"short=v" help:"Verbose output"`
	Input   string `opts:"mode=arg" help:"Input file"`
}

func (c *rootCmd) Run() error {
	if c.Verbose {
		fmt.Fprintln(os.Stderr, "processing", c.Input)
	}
	// do work...
	return nil
}

func main() {
	cli, err := opts.New(&rootCmd{}).
		Name("mytool").
		EmbedGlobalFlagSet().
		Complete().
		ParseArgsError(os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v", err)
		os.Exit(1)
	}
	if err := cli.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%s\nError: %v\n\n", cli.Selected().Help(), err)
		os.Exit(2)
	}
}
```

Layout with subcommands — a root `main.go` that owns the builder, plus one
package per command:

```go
package main

import (
	"fmt"
	"os"

	"github.com/jpillora/opts"
	"github.com/example/mytool/foocmd"
	"github.com/example/mytool/barcmd"
)

var (
	rootCmd = &struct{}{} // empty root — no top-level flags
	builder = opts.New(rootCmd).
		Name("mytool").
		EmbedGlobalFlagSet().
		Complete()
)

func init() {
	builder.AddCommand(opts.New(foocmd.NewFooCmd()).Name("foo").
		Summary("Short one-line summary of what foo does."))

	builder.AddCommand(opts.New(barcmd.NewBarCmd()).Name("bar").
		Summary("Short one-line summary of what bar does."))
}

func main() {
	cli, err := builder.ParseArgsError(os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v", err)
		os.Exit(1)
	}
	if err := cli.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%s\nError: %v\n\n", cli.Selected().Help(), err)
		os.Exit(2)
	}
}
```

Notes:
- `rootCmd = &struct{}{}` is used when the root has no flags of its own. Replace with `&someStruct{}` (with a `Run()` method) if you want the bare command (no subcommand) to do work.
- `EmbedGlobalFlagSet()` lets flags registered via `flag.Var(...)` (e.g. by imported packages) show up in `--help`.
- Subcommands can be nested by chaining `AddCommand` on a parent `opts.New(...)`, which groups them under one name in `--help`.

## 2. Adding a new subcommand

Two steps: (a) create a package with the command struct + constructor + `Run()`, (b) register it in `main.go`.

### (a) The subcommand package

Convention: one package per command in its own directory beside `main.go`, exporting a `New<Name>Cmd()` constructor.

```go
// cli/mycmd/mycmd.go
package mycmd

import "fmt"

type MyCmd struct {
	// Boolean flag: --debug or -d
	Debug bool `opts:"short=d"`

	// String flag with help text: --output foo.txt or -o foo.txt
	Output string `opts:"short=o" help:"Path to write output."`

	// Positional argument (required, in declaration order)
	Input string `opts:"mode=arg" help:"Input file."`
}

// Constructor — set defaults here, not in struct field tags.
func NewMyCmd() *MyCmd {
	return &MyCmd{
		Output: "out.txt",
	}
}

// Run is invoked after parsing. Return non-nil to signal failure.
func (c *MyCmd) Run() error {
	if c.Debug {
		fmt.Println("debug on")
	}
	// do work...
	return nil
}
```

### (b) Register it in main.go

Add to the `init()` block in `main.go`:

```go
builder.AddCommand(opts.New(mycmd.NewMyCmd()).Name("my_cmd").
	Summary("Short one-line summary."))
```

And add the import:

```go
import "goohm/mycmd"
```

(The import prefix is the module path from `go.mod` — `goohm` for
`samples/goohm` — so match whatever the existing imports in `main.go` use.)

### Struct-tag cheat sheet

| Tag                    | Effect                                                       |
|------------------------|--------------------------------------------------------------|
| `opts:"short=x"`       | Add short alias `-x` for the flag.                           |
| `opts:"mode=arg"`      | Treat field as a positional arg (order = declaration order). |
| `opts:"mode=embedded"` | Inline the fields of an embedded struct as flags.            |
| `help:"..."`           | Help text shown in `--help`.                                 |
| `opts:"-"`             | Skip this field.                                             |

For custom flag types (enums, etc.), implement `Set(string) error` on the type — and optionally `Complete(string) []string` for shell completion:

```go
type format string

func (f *format) Set(s string) error {
	switch s {
	case "json", "text":
		*f = format(s)
		return nil
	}
	return fmt.Errorf("unknown format %q", s)
}

func (f format) Complete(string) []string {
	return []string{"json", "text"}
}
```

## 3. Copying a subcommand

Use this when you want a new command that starts as a near-clone of an existing one.

Steps:

1. **Copy the package directory.** `cp -r cli/oldcmd cli/newcmd`.
2. **Rename the package** in every `.go` file: `package oldcmd` → `package newcmd`.
3. **Rename the struct and constructor.** `OldCmd` → `NewCmd`, `NewOldCmd()` → `NewNewCmd()`. Use your editor's rename-symbol or `grep -rl OldCmd cli/newcmd | xargs sed -i '' 's/OldCmd/NewCmd/g'` — but **verify** afterwards, because identifiers like `OldCmdHelper` will also be renamed.
4. **Remove anything that doesn't apply.** Copies tend to drag along irrelevant flags, helper funcs, and imports — strip them now while the diff is small.
5. **Register the new command** in `main.go`:

   ```go
   builder.AddCommand(opts.New(newcmd.NewNewCmd()).Name("new_cmd").
       Summary("..."))
   ```

6. **Build to confirm.** `go build ./...` in the module catches missed renames and unused imports.

Watch out for:
- **Unused imports.** Removing flags often leaves orphan imports that fail the build.
- **Hard-coded paths or grammar names** inside the copied `Run()` — they were correct for `oldcmd` but probably wrong for `newcmd`.
- **Test files.** If `oldcmd/oldcmd_test.go` was copied, its references need renaming too, or it can be deleted and rewritten.
- **Subcommand name collisions** in `main.go` — `Name("new_cmd")` must be unique within its parent group.

## Style

Command structs and the `opts.New(...)` chains follow the repo's Go
conventions — see [docs/kb/go-style.md](../../../docs/kb/go-style.md). In
particular, constructors return a multi-line struct literal:

```go
func NewMyCmd() *MyCmd {
	return &MyCmd{
		Output: "out.txt",
	}
}
```

## Reference

- Library docs and examples: [github.com/jpillora/opts](https://github.com/jpillora/opts).
- `opts` is not yet a dependency of any module here; add it with
  `go get github.com/jpillora/opts` in the module that needs it.
