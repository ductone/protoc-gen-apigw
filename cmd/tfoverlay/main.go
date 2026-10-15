// Command tfoverlay merges, validates and applies the Terraform overlay
// artifacts protoc-gen-apigw emits, for the Terraform provider build and for CI.
// The artifacts are OpenAPI Overlay 1.1.0 documents.
//
//	tfoverlay merge    -spec openapi.yaml -o merged.yaml [-entities merged.entities.yaml] a.yaml b.yaml
//	tfoverlay validate -spec openapi.yaml overlay.yaml...
//	tfoverlay apply    -spec openapi.yaml -o terraform.yaml [-report report.json] [-projection-overlay p.yaml] overlay.yaml...
//
// merge writes the global part, which standard Overlay tooling can apply, and
// the entity-scoped part, which only apply can. Both merge and validate check
// that applying the global part with the Speakeasy Overlay implementation gives
// the same document tfoverlay computes. apply is the step that runs between the
// spec and the Terraform generator; the spec is never modified in place.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/ductone/protoc-gen-apigw/tfoverlay"
)

// outputPerm is the mode of files the command writes. They are build inputs
// for the invoking user, so owner-only matches the rest of the repository.
const outputPerm = 0o600

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

const usage = `usage:
  tfoverlay merge    -spec spec.yaml [-o out.yaml] [-entities entities.yaml] overlay.yaml...
  tfoverlay validate -spec spec.yaml overlay.yaml...
  tfoverlay apply    -spec spec.yaml [-o out.yaml] [-report report.json] [-projection-overlay out.yaml] overlay.yaml...
`

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	var err error
	switch args[0] {
	case "merge":
		err = runMerge(args[1:], stdout)
	case "validate":
		err = runValidate(args[1:])
	case "apply":
		err = runApply(args[1:], stdout)
	case "-h", "-help", "--help", "help":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "tfoverlay: unknown command %q\n%s", args[0], usage)
		return 2
	}
	if err != nil {
		fmt.Fprintf(stderr, "tfoverlay %s: %v\n", args[0], err)
		return 1
	}
	return 0
}

func newFlags(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

func loadOverlays(paths []string) ([]*tfoverlay.Overlay, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("at least one overlay file is required")
	}
	overlays := make([]*tfoverlay.Overlay, 0, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		overlay, err := tfoverlay.Parse(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		if overlay.Source == "" {
			overlay.Source = path
		}
		overlays = append(overlays, overlay)
	}
	return overlays, nil
}

func writeOutput(path string, stdout io.Writer, data []byte) error {
	if path == "" || path == "-" {
		_, err := stdout.Write(data)
		return err
	}
	return os.WriteFile(path, data, outputPerm)
}

func runMerge(args []string, stdout io.Writer) error {
	fs := newFlags("merge")
	spec := fs.String("spec", "", "OpenAPI document the overlays apply to")
	out := fs.String("o", "", "global part output file (default stdout)")
	entities := fs.String("entities", "", "entity-scoped part output file; required when there is one")
	if err := fs.Parse(args); err != nil {
		return err
	}
	doc, merged, err := loadMerged(*spec, fs.Args())
	if err != nil {
		return err
	}
	if err := tfoverlay.VerifyStandard(doc, merged.Global()); err != nil {
		return err
	}
	rendered, err := merged.Render(doc)
	if err != nil {
		return err
	}
	if rendered.Entities != nil {
		if *entities == "" {
			return fmt.Errorf("the merged overlay has entity-scoped operations; pass -entities")
		}
		if err := os.WriteFile(*entities, rendered.Entities, outputPerm); err != nil {
			return err
		}
	}
	return writeOutput(*out, stdout, rendered.Global)
}

// loadMerged reads the spec and the merged overlay.
func loadMerged(spec string, paths []string) ([]byte, *tfoverlay.Overlay, error) {
	if spec == "" {
		return nil, nil, fmt.Errorf("-spec is required")
	}
	doc, err := os.ReadFile(spec)
	if err != nil {
		return nil, nil, err
	}
	overlays, err := loadOverlays(paths)
	if err != nil {
		return nil, nil, err
	}
	merged, err := tfoverlay.Merge(overlays...)
	if err != nil {
		return nil, nil, err
	}
	return doc, merged, nil
}

func runValidate(args []string) error {
	fs := newFlags("validate")
	spec := fs.String("spec", "", "OpenAPI document the overlays apply to")
	if err := fs.Parse(args); err != nil {
		return err
	}
	doc, merged, err := loadMerged(*spec, fs.Args())
	if err != nil {
		return err
	}
	if err := tfoverlay.ValidateTerraform(doc, merged.Operations); err != nil {
		return err
	}
	if err := tfoverlay.VerifyStandard(doc, merged.Global()); err != nil {
		return err
	}
	// A projection that fails is a failure of the overlay, so validation runs it
	// and discards the result.
	_, _, err = tfoverlay.ApplyTerraform(doc, merged.Operations)
	return err
}

func runApply(args []string, stdout io.Writer) error {
	fs := newFlags("apply")
	spec := fs.String("spec", "", "OpenAPI document the overlays apply to")
	out := fs.String("o", "", "output file (default stdout)")
	report := fs.String("report", "", "write the projection report as JSON to this file")
	projection := fs.String("projection-overlay", "", "write the change from spec to output as a standard Overlay to this file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	doc, merged, err := loadMerged(*spec, fs.Args())
	if err != nil {
		return err
	}
	result, projectionReport, err := tfoverlay.ApplyTerraform(doc, merged.Operations)
	if err != nil {
		return err
	}
	if *projection != "" {
		data, err := tfoverlay.ProjectionOverlay("Terraform projection of "+*spec, doc, result)
		if err != nil {
			return err
		}
		if err := os.WriteFile(*projection, data, outputPerm); err != nil {
			return err
		}
	}
	if *report != "" {
		data, err := json.MarshalIndent(projectionReport, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(*report, append(data, '\n'), outputPerm); err != nil {
			return err
		}
	}
	return writeOutput(*out, stdout, result)
}
