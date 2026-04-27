/*
Copyright 2025 The kcp Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// patch-upstream transforms upstream k8s.io/client-go/tools/cache source
// files into kcp-patched versions with cluster-aware key functions.
//
// Usage:
//
//	patch-upstream --upstream-dir=<path-to-client-go/tools/cache> --output-dir=<path-to-apimachinery/third_party>
package main

import (
	"flag"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
)

type fileTransform struct {
	upstreamFile string // filename inside upstream-dir
	outputSubdir string // subdir under output-dir (informers or reflector)
	outputFile   string // output filename
	transform    func(fset *token.FileSet, src []byte) ([]byte, error)
}

func main() {
	upstreamDir := flag.String("upstream-dir", "", "path to k8s.io/client-go/tools/cache/ directory")
	outputDir := flag.String("output-dir", "", "path to apimachinery/third_party/ directory")
	dryRun := flag.Bool("dry-run", false, "print to stdout instead of writing files")
	flag.Parse()

	if *upstreamDir == "" || *outputDir == "" {
		fmt.Fprintln(os.Stderr, "usage: patch-upstream --upstream-dir=<path> --output-dir=<path>")
		os.Exit(1)
	}

	transforms := []fileTransform{
		{
			upstreamFile: "shared_informer.go",
			outputSubdir: "informers",
			outputFile:   "shared_informer.go",
			transform:    transformSharedInformer,
		},
		{
			upstreamFile: "controller.go",
			outputSubdir: "reflector",
			outputFile:   "controller.go",
			transform:    transformController,
		},
		{
			upstreamFile: "reflector.go",
			outputSubdir: "reflector",
			outputFile:   "reflector.go",
			transform:    transformReflector,
		},
	}

	// Read controller.go for extracting processDeltas/processDeltasInBatch
	// into shared_informer.go.
	controllerSrc, err := os.ReadFile(filepath.Join(*upstreamDir, "controller.go"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "reading controller.go for processDeltas extraction: %v\n", err)
		os.Exit(1)
	}
	controllerFset := token.NewFileSet()
	controllerAST, err := parser.ParseFile(controllerFset, "controller.go", controllerSrc, parser.ParseComments)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parsing controller.go: %v\n", err)
		os.Exit(1)
	}
	upstreamControllerFset = controllerFset
	upstreamControllerAST = controllerAST

	for _, ft := range transforms {
		src, err := os.ReadFile(filepath.Join(*upstreamDir, ft.upstreamFile))
		if err != nil {
			fmt.Fprintf(os.Stderr, "reading %s: %v\n", ft.upstreamFile, err)
			os.Exit(1)
		}

		fset := token.NewFileSet()
		out, err := ft.transform(fset, src)
		if err != nil {
			fmt.Fprintf(os.Stderr, "transforming %s: %v\n", ft.upstreamFile, err)
			os.Exit(1)
		}

		if *dryRun {
			fmt.Printf("=== %s/%s ===\n", ft.outputSubdir, ft.outputFile)
			os.Stdout.Write(out)
			fmt.Println()
			continue
		}

		outPath := filepath.Join(*outputDir, ft.outputSubdir, ft.outputFile)
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "creating directory for %s: %v\n", outPath, err)
			os.Exit(1)
		}
		if err := os.WriteFile(outPath, out, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "writing %s: %v\n", outPath, err)
			os.Exit(1)
		}
		fmt.Printf("wrote %s\n", outPath)
	}
}
