package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/dan-strohschein/cartograph/pkg/graph"
	"github.com/dan-strohschein/cartograph/pkg/loader"
	"github.com/dan-strohschein/cartograph/pkg/output"
	"github.com/dan-strohschein/cartograph/pkg/query"
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
	noCache := fs.Bool("no-cache", false, "Disable graph cache (always re-parse AID files)")

	// Find the subcommand by scanning past any leading global flags.
	// This allows both "cartograph depends Foo --dir /p" and "cartograph --dir /p depends Foo".
	subcommandIdx := 1
	knownValueFlags := map[string]bool{"--dir": true, "--format": true, "--depth": true, "-dir": true, "-format": true, "-depth": true}
	for subcommandIdx < len(os.Args) {
		arg := os.Args[subcommandIdx]
		if knownValueFlags[arg] {
			subcommandIdx += 2 // skip flag + value
			continue
		}
		if strings.HasPrefix(arg, "-") {
			// Unknown flag before subcommand — skip it
			subcommandIdx++
			continue
		}
		break
	}
	if subcommandIdx >= len(os.Args) {
		printUsage()
		os.Exit(1)
	}
	subcommand := os.Args[subcommandIdx]

	// Rebuild os.Args[2:] equivalent: everything except the program name and subcommand.
	var restArgs []string
	for i := 1; i < len(os.Args); i++ {
		if i == subcommandIdx {
			continue
		}
		restArgs = append(restArgs, os.Args[i])
	}

	// Handle stats subcommand separately (no positional arg).
	if subcommand == "stats" {
		fs.Parse(restArgs)
		g, err := loadGraph(*dir, *noCache)
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
	args := restArgs
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
			} else if name == "no-cache" {
				flagArgs = append(flagArgs, args[i])
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

	g, err := loadGraph(*dir, *noCache)
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
		for _, a := range restArgs {
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

	case "search":
		pattern := remaining[0]
		var kindFilter graph.NodeKind
		for i, arg := range remaining[1:] {
			if arg == "--kind" && i+1 < len(remaining[1:]) {
				v := remaining[i+2]
				if len(v) > 0 {
					v = strings.ToUpper(v[:1]) + v[1:]
				}
				kindFilter = graph.NodeKind(v)
			}
		}
		result, err := engine.Search(pattern, kindFilter)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		renderSearch(result)

	case "list":
		moduleName := remaining[0]
		result, err := engine.ListModule(moduleName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		renderSearch(result)

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

// loadGraph loads AID files into a graph. When dir is specified, it loads from
// that directory directly. Otherwise, it uses aidkit's discovery protocol.
// When noCache is false, uses the gob cache for faster subsequent loads.
func loadGraph(dir string, noCache bool) (*graph.Graph, error) {
	if dir != "" {
		if noCache {
			return loader.LoadFromDirectory(dir)
		}
		return loader.LoadFromDirectoryCached(dir)
	}
	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("cannot get working directory: %w", err)
	}
	g, _, err := loader.LoadWithDiscovery(wd)
	if err != nil {
		return nil, err
	}
	if g == nil {
		return nil, fmt.Errorf("no .aidocs/ directory found (searched up from %s)", wd)
	}
	return g, nil
}

func renderSearch(result *query.SearchResult) {
	fmt.Printf("Search: %q — %d match(es)\n\n", result.Pattern, result.Total)
	kindOrder := []graph.NodeKind{
		graph.KindModule, graph.KindType, graph.KindTrait,
		graph.KindFunction, graph.KindMethod, graph.KindField,
		graph.KindConstant, graph.KindWorkflow, graph.KindLock,
	}
	for _, kind := range kindOrder {
		nodes, ok := result.Matches[kind]
		if !ok || len(nodes) == 0 {
			continue
		}
		fmt.Printf("  %s (%d):\n", kind, len(nodes))
		for _, n := range nodes {
			loc := ""
			if n.SourceFile != "" {
				loc = fmt.Sprintf(" (%s:%d)", n.SourceFile, n.SourceLine)
			}
			purpose := ""
			if n.Purpose != "" {
				purpose = " — " + n.Purpose
			}
			fmt.Printf("    %s%s%s\n", n.Name, purpose, loc)
		}
		fmt.Println()
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Cartograph — semantic code index from AID files

Usage:
  cartograph errors <ErrorType>                    What produces this error?
  cartograph field <Type.Field>                    What touches this field?
  cartograph callstack <function> [--up|--down]    What's the call stack?
  cartograph depends <Type>                        What depends on this type?
  cartograph effects <function>                    What are the side effects?
  cartograph search <pattern> [--kind <kind>]      Find nodes by name (glob/regex)
  cartograph list <module>                         List all nodes in a module
  cartograph stats                                 Show graph statistics

  Methods use Type.Method format: cartograph callstack DB.Compact --down

Flags:
  --dir <path>     Path to .aid files directory (default: auto-discover .aidocs/)
  --format <fmt>   Output format: tree (default), json
  --depth <n>      Max traversal depth, 1-50 (default: 10)
  --no-cache       Disable graph cache (always re-parse AID files)
`)
}
