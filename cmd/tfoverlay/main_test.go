package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ductone/protoc-gen-apigw/tfoverlay"
)

var testdata = filepath.Join("..", "..", "tfoverlay", "testdata", "iga4347")

func runCLI(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestApplyMatchesTheLibrary(t *testing.T) {
	spec := filepath.Join(testdata, "spec.yaml")
	overlay := filepath.Join(testdata, "scoped.terraform_entity_overlay.yaml")
	reportPath := filepath.Join(t.TempDir(), "report.json")

	projectionPath := filepath.Join(t.TempDir(), "projection.yaml")
	code, stdout, stderr := runCLI(t, "apply", "-spec", spec, "-report", reportPath, "-projection-overlay", projectionPath, overlay)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}

	doc, err := os.ReadFile(spec)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(overlay)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := tfoverlay.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	want, _, err := tfoverlay.ApplyTerraform(doc, parsed.Operations)
	if err != nil {
		t.Fatal(err)
	}
	if stdout != string(want) {
		t.Fatal("CLI output differs from ApplyTerraform")
	}

	projection, err := os.ReadFile(projectionPath)
	if err != nil || !strings.Contains(string(projection), "actions:") {
		t.Fatalf("projection overlay = %s (%v)", projection, err)
	}

	var report tfoverlay.ProjectionReport
	raw, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &report); err != nil || len(report.Entities) != 1 || len(report.Entities[0].Clones) != 2 {
		t.Fatalf("report = %s (%v)", raw, err)
	}
}

func TestValidateAndFailures(t *testing.T) {
	spec := filepath.Join(testdata, "spec.yaml")
	overlay := filepath.Join(testdata, "scoped.terraform_entity_overlay.yaml")
	if code, _, stderr := runCLI(t, "validate", "-spec", spec, overlay); code != 0 {
		t.Fatalf("validate exit %d: %s", code, stderr)
	}

	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.yaml")
	source, _ := os.ReadFile(overlay)
	if err := os.WriteFile(bad, []byte(strings.Replace(string(source), "Custom App Entitlement", "Unknown Entity", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := runCLI(t, "validate", "-spec", spec, bad); code != 1 || !strings.Contains(stderr, "no operation of the entity") {
		t.Fatalf("validate of an unknown entity: exit %d, %q", code, stderr)
	}
	if code, _, stderr := runCLI(t, "apply", overlay); code != 1 || !strings.Contains(stderr, "-spec is required") {
		t.Fatalf("apply without a spec: exit %d, %q", code, stderr)
	}
	if code, _, _ := runCLI(t); code != 2 {
		t.Fatalf("no arguments: exit %d, want 2", code)
	}
	if code, _, _ := runCLI(t, "nope"); code != 2 {
		t.Fatalf("unknown command: exit %d, want 2", code)
	}
}

func TestMerge(t *testing.T) {
	spec := filepath.Join(testdata, "spec.yaml")
	overlay := filepath.Join(testdata, "scoped.terraform_entity_overlay.yaml")
	if code, _, stderr := runCLI(t, "merge", "-spec", spec, overlay, overlay); code != 1 || !strings.Contains(stderr, "pass -entities") {
		t.Fatalf("merge of entity operations without -entities: exit %d, %q", code, stderr)
	}
	entities := filepath.Join(t.TempDir(), "entities.yaml")
	code, stdout, stderr := runCLI(t, "merge", "-spec", spec, "-entities", entities, overlay, overlay)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if stdout != "" {
		t.Fatalf("an overlay of entity-scoped operations has no global part:\n%s", stdout)
	}
	want, _ := os.ReadFile(overlay)
	got, err := os.ReadFile(entities)
	if err != nil || strings.Count(string(got), "- target:") != strings.Count(string(want), "- target:") {
		t.Fatalf("merged entity part (%v):\n%s", err, got)
	}
	if code, _, stderr := runCLI(t, "merge", overlay); code != 1 || !strings.Contains(stderr, "-spec is required") {
		t.Fatalf("merge without a spec: exit %d, %q", code, stderr)
	}
}
