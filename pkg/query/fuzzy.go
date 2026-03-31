package query

import (
	"sort"
	"strings"

	"github.com/dan-strohschein/cartograph/pkg/graph"
)

// suggestion is a candidate match with a score (lower is better).
type suggestion struct {
	Name  string
	Score int
}

// suggestSimilar finds the top N most similar node names to the query.
func (qe *QueryEngine) suggestSimilar(name string, kinds []graph.NodeKind, maxResults int) []string {
	kindSet := make(map[graph.NodeKind]bool)
	for _, k := range kinds {
		kindSet[k] = true
	}
	filterKinds := len(kinds) > 0

	var candidates []suggestion
	nameLower := strings.ToLower(name)

	for _, n := range qe.g.AllNodes() {
		if filterKinds && !kindSet[n.Kind] {
			continue
		}
		if n.Kind == graph.KindField || n.Kind == graph.KindModule {
			continue
		}

		nodeLower := strings.ToLower(n.Name)

		// Substring match (strongest signal).
		if strings.Contains(nodeLower, nameLower) || strings.Contains(nameLower, nodeLower) {
			candidates = append(candidates, suggestion{Name: n.Name, Score: 0})
			continue
		}

		// Method expansion: bare name "Compact" matches "DB.Compact".
		if strings.Contains(n.Name, ".") {
			parts := strings.SplitN(n.Name, ".", 2)
			if len(parts) == 2 && strings.EqualFold(parts[1], name) {
				candidates = append(candidates, suggestion{Name: n.Name, Score: 1})
				continue
			}
		}

		// Levenshtein distance.
		dist := levenshtein(nameLower, nodeLower)
		threshold := len(name) / 3
		if threshold < 3 {
			threshold = 3
		}
		if dist <= threshold {
			candidates = append(candidates, suggestion{Name: n.Name, Score: dist + 2})
		}
	}

	// Deduplicate.
	seen := make(map[string]bool)
	var unique []suggestion
	for _, c := range candidates {
		if !seen[c.Name] {
			seen[c.Name] = true
			unique = append(unique, c)
		}
	}

	// Sort by score, then alphabetically.
	sort.Slice(unique, func(i, j int) bool {
		if unique[i].Score != unique[j].Score {
			return unique[i].Score < unique[j].Score
		}
		return unique[i].Name < unique[j].Name
	})

	if len(unique) > maxResults {
		unique = unique[:maxResults]
	}

	result := make([]string, len(unique))
	for i, s := range unique {
		result[i] = s.Name
	}
	return result
}

// levenshtein computes the edit distance between two strings.
func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(curr[j-1]+1, min(prev[j]+1, prev[j-1]+cost))
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}
