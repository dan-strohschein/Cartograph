package query

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/dan-strohschein/cartograph/pkg/graph"
)

// SearchResult groups matched nodes by kind.
type SearchResult struct {
	Pattern  string
	Matches  map[graph.NodeKind][]graph.Node
	Total    int
}

// Search finds all nodes whose name matches a pattern (supports regex and * glob).
func (qe *QueryEngine) Search(pattern string, kindFilter graph.NodeKind) (*SearchResult, error) {
	// Convert simple glob to regex: * → .*, ? → .
	regexPattern := pattern
	if !strings.ContainsAny(pattern, "^$()[]{}|\\+") {
		// Looks like a glob, not a regex — convert it
		regexPattern = strings.ReplaceAll(regexPattern, ".", "\\.")
		regexPattern = strings.ReplaceAll(regexPattern, "*", ".*")
		regexPattern = strings.ReplaceAll(regexPattern, "?", ".")
	}
	re, err := regexp.Compile("(?i)" + regexPattern)
	if err != nil {
		return nil, fmt.Errorf("invalid pattern: %w", err)
	}

	result := &SearchResult{
		Pattern: pattern,
		Matches: make(map[graph.NodeKind][]graph.Node),
	}

	for _, n := range qe.g.AllNodes() {
		if kindFilter != "" && n.Kind != kindFilter {
			continue
		}
		if re.MatchString(n.Name) || re.MatchString(n.QualifiedName) {
			result.Matches[n.Kind] = append(result.Matches[n.Kind], n)
			result.Total++
		}
	}

	// Sort each group by name.
	for kind := range result.Matches {
		sort.Slice(result.Matches[kind], func(i, j int) bool {
			return result.Matches[kind][i].Name < result.Matches[kind][j].Name
		})
	}

	return result, nil
}

// ListModule returns all nodes in a specific module, grouped by kind.
func (qe *QueryEngine) ListModule(moduleName string) (*SearchResult, error) {
	result := &SearchResult{
		Pattern: moduleName,
		Matches: make(map[graph.NodeKind][]graph.Node),
	}

	for _, n := range qe.g.AllNodes() {
		if n.Module == moduleName {
			result.Matches[n.Kind] = append(result.Matches[n.Kind], n)
			result.Total++
		}
	}

	if result.Total == 0 {
		// Try partial match on module name.
		for _, n := range qe.g.AllNodes() {
			if strings.Contains(n.Module, moduleName) {
				result.Matches[n.Kind] = append(result.Matches[n.Kind], n)
				result.Total++
			}
		}
	}

	if result.Total == 0 {
		return nil, &NotFoundError{Entity: moduleName, Kind: "module"}
	}

	// Sort each group by name.
	for kind := range result.Matches {
		sort.Slice(result.Matches[kind], func(i, j int) bool {
			return result.Matches[kind][i].Name < result.Matches[kind][j].Name
		})
	}

	return result, nil
}
