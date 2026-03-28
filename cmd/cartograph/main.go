package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dan-strohschein/cartograph/internal/loader"
	"github.com/dan-strohschein/cartograph/internal/output"
	"github.com/dan-strohschein/cartograph/internal/query"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Global flags.
	fs := flag.NewFlagSet("cartograph", flag.ExitOnError)
	dir := fs.String("dir", "", "Path to directory containing .aid files (default: auto-discover .aidocs/)")
	format := fs.String("format", "tree", "Output format: tree, json")
	depth := fs.Int("depth", 10, "Max traversal depth (1-50)")

	subcommand := os.Args[1]

	// Handle stats subcommand separately (no positional arg).
	if subcommand == "stats" {
		fs.Parse(os.Args[2:])
		aidDir := resolveDir(*dir)
		g, err := loader.LoadFromDirectory(aidDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		stats := g.Stats()
		fmt.Printf("Graph Statistics\n")
		fmt.Printf("  Nodes: %d\n", stats.NodeCount)
		fmt.Printf("  Edges: %d\n", stats.EdgeCount)
		fmt.Printf("  Modules: %d\n", stats.Modules)
		fmt.Printf("  Nodes by kind:\n")
		for k, v := range stats.NodesByKind {
			fmt.Printf("    %s: %d\n", k, v)
		}
		fmt.Printf("  Edges by kind:\n")
		for k, v := range stats.EdgesByKind {
			fmt.Printf("    %s: %d\n", k, v)
		}
		return
	}

	// Separate flags from positional args manually, since Go's flag package
	// stops at the first non-flag arg.
	var flagArgs, positionalArgs []string
	args := os.Args[2:]
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "--") || strings.HasPrefix(args[i], "-") {
			name := strings.TrimLeft(args[i], "-")
			// Known value flags go to the flag parser.
			if name == "dir" || name == "format" || name == "depth" {
				flagArgs = append(flagArgs, args[i])
				if i+1 < len(args) {
					i++
					flagArgs = append(flagArgs, args[i])
				}
			} else {
				// Direction flags (--up, --down, --both) and unknown flags go to positional.
				positionalArgs = append(positionalArgs, args[i])
			}
		} else {
			positionalArgs = append(positionalArgs, args[i])
		}
	}
	fs.Parse(flagArgs)

	if len(positionalArgs) == 0 {
		fmt.Fprintf(os.Stderr, "Error: %s requires a target argument\n", subcommand)
		printUsage()
		os.Exit(1)
	}
	remaining := positionalArgs

	aidDir := resolveDir(*dir)

	g, err := loader.LoadFromDirectory(aidDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading AID files: %v\n", err)
		os.Exit(1)
	}

	engine := query.NewQueryEngine(g, *depth)

	switch subcommand {
	case "errors":
		errorType := remaining[0]
		result, err := engine.ErrorProducers(errorType)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		render(*format, result, nil)

	case "field":
		target := remaining[0]
		parts := strings.SplitN(target, ".", 2)
		if len(parts) != 2 {
			fmt.Fprintf(os.Stderr, "Error: field requires Type.Field format (e.g., User.email)\n")
			os.Exit(1)
		}
		result, err := engine.FieldTouchers(parts[0], parts[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		render(*format, result, nil)

	case "callstack":
		funcName := remaining[0]
		direction := query.Both
		for _, arg := range remaining[1:] {
			switch arg {
			case "--up":
				direction = query.Reverse
			case "--down":
				direction = query.Forward
			case "--both":
				direction = query.Both
			}
		}
		// Also check flags parsed before positional args.
		for _, a := range os.Args[2:] {
			switch a {
			case "--up":
				direction = query.Reverse
			case "--down":
				direction = query.Forward
			case "--both":
				direction = query.Both
			}
		}
		result, err := engine.CallStack(funcName, direction)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		render(*format, result, nil)

	case "depends":
		typeName := remaining[0]
		result, err := engine.TypeDependents(typeName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		render(*format, result, nil)

	case "effects":
		funcName := remaining[0]
		report, err := engine.SideEffects(funcName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		render(*format, nil, report)

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", subcommand)
		printUsage()
		os.Exit(1)
	}
}

func render(format string, result *query.QueryResult, effects *query.EffectReport) {
	switch format {
	case "json":
		if result != nil {
			output.RenderJSON(os.Stdout, result)
		} else if effects != nil {
			output.RenderEffectJSON(os.Stdout, effects)
		}
	default:
		if result != nil {
			output.RenderTree(os.Stdout, result)
		} else if effects != nil {
			output.RenderEffectTree(os.Stdout, effects)
		}
	}
}

func resolveDir(dir string) string {
	if dir != "" {
		return dir
	}
	// Walk up looking for .aidocs/.
	wd, err := os.Getwd()
	if err != nil {
		return ".aidocs"
	}
	for d := wd; ; d = filepath.Dir(d) {
		candidate := filepath.Join(d, ".aidocs")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
	}
	// If no .aidocs/ found, try current directory (flat .aid files).
	return wd
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Cartograph — semantic code index from AID files

Usage:
  cartograph errors <ErrorType>                    What produces this error?
  cartograph field <Type.Field>                    What touches this field?
  cartograph callstack <function> [--up|--down]    What's the call stack?
  cartograph depends <Type>                        What depends on this type?
  cartograph effects <function>                    What are the side effects?
  cartograph stats                                 Show graph statistics

Flags:
  --dir <path>     Path to .aid files directory (default: auto-discover .aidocs/)
  --format <fmt>   Output format: tree (default), json
  --depth <n>      Max traversal depth, 1-50 (default: 10)
`)
}
