package loader

import (
	"path/filepath"
	"testing"
)

// manifestCoverageDir is testdata for LoadWithManifest (transitive @depends).
func manifestCoverageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join("testdata", "manifest_coverage")
}

func TestLoadWithManifestTransitiveDepends(t *testing.T) {
	dir := manifestCoverageDir(t)
	manifestPath := filepath.Join(dir, "manifest.aid")

	g, err := LoadWithManifest(manifestPath, []string{"fmtcov/leaf"})
	if err != nil {
		t.Fatalf("LoadWithManifest: %v", err)
	}

	leafFns := g.NodesByName("LeafFn")
	if len(leafFns) == 0 {
		t.Fatal("expected LeafFn from leaf.aid")
	}
	coreFns := g.NodesByName("CoreHelper")
	if len(coreFns) == 0 {
		t.Fatal("expected CoreHelper from core.aid (transitive via @depends)")
	}
}

func TestLoadWithManifestLeafOnlyVsFull(t *testing.T) {
	dir := manifestCoverageDir(t)
	leafOnly := filepath.Join(dir, "leaf.aid")

	gLeaf, err := LoadFromFiles([]string{leafOnly})
	if err != nil {
		t.Fatalf("LoadFromFiles(leaf): %v", err)
	}
	if len(gLeaf.NodesByName("CoreHelper")) != 0 {
		t.Error("loading leaf.aid alone should not define CoreHelper node")
	}

	manifestPath := filepath.Join(dir, "manifest.aid")
	gFull, err := LoadWithManifest(manifestPath, []string{"fmtcov/leaf"})
	if err != nil {
		t.Fatalf("LoadWithManifest: %v", err)
	}
	if len(gFull.NodesByName("CoreHelper")) == 0 {
		t.Fatal("manifest load should include transitive core.aid")
	}
	if gFull.Stats().NodeCount <= gLeaf.Stats().NodeCount {
		t.Errorf("manifest graph should be strictly larger than leaf-only graph (full=%d leaf=%d)",
			gFull.Stats().NodeCount, gLeaf.Stats().NodeCount)
	}
}

func TestLoadWithManifestEmptyPackages(t *testing.T) {
	dir := manifestCoverageDir(t)
	manifestPath := filepath.Join(dir, "manifest.aid")

	_, err := LoadWithManifest(manifestPath, nil)
	if err == nil {
		t.Fatal("expected error for empty relevantPackages")
	}

	_, err = LoadWithManifest(manifestPath, []string{})
	if err == nil {
		t.Fatal("expected error for empty relevantPackages slice")
	}
}

func TestLoadWithManifestInvalidPath(t *testing.T) {
	_, err := LoadWithManifest(filepath.Join(manifestCoverageDir(t), "nonexistent-manifest.aid"), []string{"fmtcov/leaf"})
	if err == nil {
		t.Fatal("expected error for missing manifest path")
	}
}

func TestLoadWithManifestNotManifestFile(t *testing.T) {
	// Regular module AID must be rejected.
	leafPath := filepath.Join(manifestCoverageDir(t), "leaf.aid")
	_, err := LoadWithManifest(leafPath, []string{"fmtcov/leaf"})
	if err == nil {
		t.Fatal("expected error when manifest path is not a manifest file")
	}
}
