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

// transformReflector transforms upstream reflector.go into the kcp reflector/reflector.go.
func transformReflector(fset *token.FileSet, src []byte) ([]byte, error) {
	file, err := parser.ParseFile(fset, "reflector.go", src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parsing: %w", err)
	}

	// 1. Change package.
	changePackageName(file, "reflector")

	// 2. Add kcp imports.
	addImport(file, "", "k8s.io/client-go/tools/cache")
	addImport(file, "kcpcache", "github.com/kcp-dev/apimachinery/v2/pkg/cache")

	// 3. Qualify bare identifiers with cache. prefix.
	// The reflector file defines Reflector, ReflectorOptions locally,
	// and also defines local WatchErrorHandlerWithContext, VeryShortWatchError, etc.
	localDefs := toSet(
		"Reflector", "ReflectorOptions",
		"WatchErrorHandlerWithContext", "DefaultWatchErrorHandler",
		"VeryShortWatchError",
		"handleListWatch", "handleWatch", "handleAnyWatch",
		"defaultExpectedTypeName", "defaultMinWatchTimeout",
		"errorStopRequested", "internalPackages",
		"getTypeDescriptionFromObject", "getExpectedGVKFromObject",
		"isExpiredError", "isTooLargeResourceVersionError", "isWatchErrorRetriable",
		"initialEventsEndBookmarkTicker", "newInitialEventsEndBookmarkTicker",
		"newInitialEventsEndBookmarkTickerInternal",
		"noopTicker",
		"isUnsupportedTableObject", "unsupportedTableGVK",
		"checkWatchListDataConsistencyIfRequested",
	)
	qualifyIdents(file, "cache", cacheTypesToQualify, localDefs)

	// 4. Render AST to source.
	out, err := renderFile(fset, file)
	if err != nil {
		return nil, err
	}

	// 5. Text-level kcp modifications.

	// 5a. Add keyFunction field to Reflector struct.
	out = replaceInSource(out,
		"useWatchList bool",
		"useWatchList bool\n\n\t// kcp modification: keyFunction is used to generate keys for objects in the temporary store\n\t// during WatchList operations.\n\tkeyFunction cache.KeyFunc")

	// 5b. Add KeyFunction field to ReflectorOptions struct.
	out = replaceInSource(out,
		"Clock clock.Clock",
		"Clock clock.Clock\n\n\t// kcp modification: KeyFunction is used to generate keys for objects in the temporary store.\n\t// If unset, defaults to DeletionHandlingMetaClusterNamespaceKeyFunc.\n\tKeyFunction cache.KeyFunc")

	// 5c. In NewReflectorWithOptions, default keyFunction and add it to the struct literal.
	out = replaceInSource(out,
		"r := &Reflector{",
		`// kcp modification: default to cluster-aware key function
	keyFunction := options.KeyFunction
	if keyFunction == nil {
		keyFunction = kcpcache.DeletionHandlingMetaClusterNamespaceKeyFunc
	}

	r := &Reflector{`)

	out = replaceInSource(out,
		"expectedType:      reflect.TypeOf(expectedType),",
		"expectedType:      reflect.TypeOf(expectedType),\n\t\tkeyFunction:       keyFunction, // kcp modification")

	// 5d. Change local WatchErrorHandlerWithContext to use local *Reflector.
	// The upstream type uses *cache.Reflector, but we need *Reflector (local).
	// We add a local type alias and fix the default handler.
	out = replaceInSource(out,
		"watchErrorHandler cache.WatchErrorHandlerWithContext",
		"// kcp modification: use local type instead of cache.WatchErrorHandlerWithContext\n\twatchErrorHandler WatchErrorHandlerWithContext")

	// 5e. Use local DefaultWatchErrorHandler.
	out = replaceInSource(out,
		"watchErrorHandler: cache.DefaultWatchErrorHandler,",
		"watchErrorHandler: DefaultWatchErrorHandler, // kcp modification")

	// 5f. In watchList, replace NewStore key function.
	out = replaceInSource(out,
		"cache.NewStore(cache.DeletionHandlingMetaNamespaceKeyFunc, storeOpts...)",
		"// kcp modification: use cluster-aware key function\n\t\tcache.NewStore(r.keyFunction, storeOpts...)")

	// 5g. Add local WatchErrorHandlerWithContext type and DefaultWatchErrorHandler.
	// These need to reference the local *Reflector type.
	// Find and replace the upstream WatchErrorHandlerWithContext references.
	// The upstream file doesn't define WatchErrorHandlerWithContext (it's in another file),
	// so we need to add it.
	watchErrorHandlerType := `
// WatchErrorHandlerWithContext is called whenever ListAndWatch drops the
// connection with an error. After calling this handler, the informer
// will backoff and retry.
// kcp modification: uses local *Reflector instead of *cache.Reflector.
type WatchErrorHandlerWithContext func(ctx context.Context, r *Reflector, err error)

// DefaultWatchErrorHandler is the default implementation of WatchErrorHandlerWithContext.
func DefaultWatchErrorHandler(ctx context.Context, r *Reflector, err error) {
	switch {
	case isExpiredError(err):
		klog.FromContext(ctx).V(4).Info("Watch closed", "reflector", r.name, "type", r.typeDescription, "err", err)
	case err == io.EOF:
		// watch closed normally
	case err == io.ErrUnexpectedEOF:
		klog.FromContext(ctx).V(1).Info("Watch closed with unexpected EOF", "reflector", r.name, "type", r.typeDescription, "err", err)
	default:
		utilruntime.HandleErrorWithContext(ctx, err, "Failed to watch", "reflector", r.name, "type", r.typeDescription)
	}
}

`
	// Insert after the defaultMinWatchTimeout variable.
	out = insertAfterLine(out, "var defaultMinWatchTimeout", watchErrorHandlerType)

	// 5h. Remove upstream's DefaultWatchErrorHandler if it was pulled in via cache. qualification.
	// (It won't be — it's defined in another upstream file. But we need to make sure
	// our local one is used.)

	// 5i. Update internalPackages to include our package path.
	out = replaceInSource(out,
		`var internalPackages = []string{"client-go/tools/cache/"}`,
		`var internalPackages = []string{"client-go/tools/cache/", "apimachinery/third_party/reflector/"}`)

	// 5j. Add stub for checkWatchListDataConsistencyIfRequested.
	// In upstream this is defined in a separate file within the cache package.
	// Since it's unexported and uses internal types, we provide a no-op stub.
	stub := `
// checkWatchListDataConsistencyIfRequested performs a data consistency check against the
// temporary store. In the forked version, we skip this as the main fix is using
// cluster-aware key functions.
// kcp modification: no-op stub for unexported upstream function.
func checkWatchListDataConsistencyIfRequested(ctx context.Context, name string, resourceVersion string, listFunc func(ctx context.Context, options metav1.ListOptions) (runtime.Object, error), transformer cache.TransformFunc, storeListFunc func() []interface{}) {
}
`
	out = append(out, []byte(stub)...)

	// 6. Add copyright modification.
	out = addCopyrightModification(out)

	// 7. Final format.
	out, err = formatSource(out)
	if err != nil {
		return nil, fmt.Errorf("final format: %w", err)
	}

	return out, nil
}
