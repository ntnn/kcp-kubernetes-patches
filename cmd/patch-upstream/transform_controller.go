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

package main

import (
	"fmt"
	"go/parser"
	"go/token"
)

// transformController transforms upstream controller.go into the kcp reflector/controller.go.
// This is the most aggressively trimmed file: we only keep Config, controller, New,
// Run, RunWithContext, processLoop, HasSynced, LastSyncResourceVersion.
func transformController(fset *token.FileSet, src []byte) ([]byte, error) {
	file, err := parser.ParseFile(fset, "controller.go", src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parsing: %w", err)
	}

	// 1. Change package.
	changePackageName(file, "reflector")

	// 2. Remove declarations we don't need.
	removeTopLevelDecl(file, toSet(
		// Interfaces
		"Controller",
		// Types
		"ShouldResyncFunc", "ProcessFunc", "ProcessBatchFunc",
		"ResourceEventHandler",
		"ResourceEventHandlerFuncs", "ResourceEventHandlerDetailedFuncs",
		"FilteringResourceEventHandler",
		"InformerOptions",
		// Functions
		"NewInformerWithOptions", "NewInformer",
		"NewIndexerInformer", "NewTransformingInformer",
		"NewTransformingIndexerInformer",
		"processDeltas", "processDeltasInBatch",
		"newInformer", "newQueueFIFO",
		"DeletionHandlingMetaNamespaceKeyFunc", "DeletionHandlingObjectToName",
	))

	// Remove methods on types we don't keep.
	removeMethodsOnType(file, "ResourceEventHandlerFuncs", toSet("OnAdd", "OnUpdate", "OnDelete"))
	removeMethodsOnType(file, "ResourceEventHandlerDetailedFuncs", toSet("OnAdd", "OnUpdate", "OnDelete"))
	removeMethodsOnType(file, "FilteringResourceEventHandler", toSet("OnAdd", "OnUpdate", "OnDelete"))

	// 3. Add kcp imports.
	addImport(file, "", "k8s.io/client-go/tools/cache")

	// 4. Qualify bare identifiers with cache. prefix.
	localDefs := toSet(
		"Config", "controller", "New",
	)
	qualifyIdents(file, "cache", cacheTypesToQualify, localDefs)

	// 5. Render AST to source.
	out, err := renderFile(fset, file)
	if err != nil {
		return nil, err
	}

	// 6. Text-level kcp modifications.

	// 6a. Add KeyFunction field to Config struct.
	out = replaceInSource(out,
		"WatchListPageSize int64",
		"WatchListPageSize int64\n\n\t// kcp modification: KeyFunction is used to generate keys for objects\n\t// in the reflector's temporary store during WatchList operations.\n\tKeyFunction cache.KeyFunc")

	// 6b. Change New() return type to cache.Controller.
	out = replaceInSource(out,
		"func New(c *Config) cache.Controller {",
		"// kcp modification: returns cache.Controller so it can be used as a drop-in replacement.\nfunc New(c *Config) cache.Controller {")

	// 6c. In RunWithContext, use local NewReflectorWithOptions (not cache.NewReflectorWithOptions)
	// and pass KeyFunction.
	out = replaceInSource(out,
		"cache.NewReflectorWithOptions(",
		"NewReflectorWithOptions(")
	out = replaceInSource(out,
		"cache.ReflectorOptions{",
		"ReflectorOptions{")

	// 6d. Add KeyFunction to ReflectorOptions in RunWithContext.
	out = replaceInSource(out,
		"Clock:           c.clock,\n\t\t})",
		"Clock:           c.clock,\n\t\t\tKeyFunction:     c.config.KeyFunction, // kcp modification\n\t\t})")

	// 6e. Fix WatchErrorHandler callbacks to pass nil for cache.Reflector
	// (our Reflector is a different type).
	out = replaceInSource(out,
		"r.watchErrorHandler = func(_ context.Context, r *Reflector, err error) {\n\t\t\tc.config.WatchErrorHandler(r, err)",
		"r.watchErrorHandler = func(_ context.Context, refl *Reflector, err error) {\n\t\t\t// kcp modification: pass nil since our Reflector != cache.Reflector\n\t\t\tc.config.WatchErrorHandler(nil, err)")
	out = replaceInSource(out,
		"r.watchErrorHandler = c.config.WatchErrorHandlerWithContext",
		"r.watchErrorHandler = func(ctx context.Context, refl *Reflector, err error) {\n\t\t\t// kcp modification: pass nil since our Reflector != cache.Reflector\n\t\t\tc.config.WatchErrorHandlerWithContext(ctx, nil, err)\n\t\t}")

	// 6f. Fix reflector field type: should be local *Reflector, not *cache.Reflector.
	out = replaceInSource(out,
		"reflector      *cache.Reflector",
		"reflector      *Reflector")

	// 7. Add copyright modification.
	out = addCopyrightModification(out)

	// 8. Remove unused imports that result from stripping declarations.
	// We do this by attempting to format and relying on the compiler.
	// For now, leave it to goimports or manual cleanup.
	out, err = formatSource(out)
	if err != nil {
		return nil, fmt.Errorf("final format: %w", err)
	}

	return out, nil
}
