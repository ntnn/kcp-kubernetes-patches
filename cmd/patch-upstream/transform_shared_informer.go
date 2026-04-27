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
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
)

// Shared state: upstream controller.go AST, used to extract processDeltas/processDeltasInBatch.
var (
	upstreamControllerFset *token.FileSet
	upstreamControllerAST  *ast.File
)

// cacheTypesToQualify is the set of identifier names from the upstream cache package
// that need a "cache." qualifier when moved to a different package.
var cacheTypesToQualify = toSet(
	// Interfaces
	"SharedInformer", "SharedIndexInformer", "Controller", "Store",
	"Indexer", "Queue", "QueueWithBatch",
	"ListerWatcher", "ListerWatcherWithContext",
	"MutationDetector", "ResourceEventHandler",
	"ResourceEventHandlerRegistration", "ReflectorStore",
	"TransformingStore", "ResourceVersionUpdater", "TransactionStore",

	// Structs/types
	"Indexers", "Deltas", "Delta",
	"DeltaFIFOOptions", "RealFIFOOptions",
	"SharedIndexInformerOptions", "HandlerOptions",
	"TransformFunc", "WatchErrorHandler", "WatchErrorHandlerWithContext",
	"ShouldResyncFunc", "KeyFunc", "ProcessFunc", "ProcessBatchFunc",
	"Reflector", "ReflectorOptions",
	"Transaction", "TransactionError",
	"DeletedFinalStateUnknown", "ExplicitKey",
	"ResourceEventHandlerFuncs", "ResourceEventHandlerDetailedFuncs",
	"FilteringResourceEventHandler",
	"StoreOption",

	// Functions
	"NewIndexer", "NewStore", "NewDeltaFIFOWithOptions", "NewRealFIFOWithOptions",
	"NewCacheMutationDetector", "NewReflectorWithOptions", "NewRetryWithDeadline",
	"ToListerWatcherWithContext", "WithTransformer",
	"DeletionHandlingMetaNamespaceKeyFunc", "DeletionHandlingObjectToName",
	"MetaNamespaceKeyFunc",
	"PopProcessFunc",

	// Constants
	"Sync", "Replaced", "Added", "Updated", "Deleted",
	"TransactionTypeUpdate", "TransactionTypeAdd", "TransactionTypeDelete",

	// Errors
	"ErrFIFOClosed",
)

// transformSharedInformer transforms upstream shared_informer.go into the kcp version.
func transformSharedInformer(fset *token.FileSet, src []byte) ([]byte, error) {
	file, err := parser.ParseFile(fset, "shared_informer.go", src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parsing: %w", err)
	}

	// 1. Change package.
	changePackageName(file, "informers")

	// 2. Remove type/interface declarations that are already in the cache package.
	removeTopLevelDecl(file, toSet(
		"SharedInformer",
		"SharedIndexInformer",
		"ResourceEventHandlerRegistration",
		"SharedIndexInformerOptions",
	))

	// 3. Add kcp imports.
	addImport(file, "kcpcache", "github.com/kcp-dev/apimachinery/v2/pkg/cache")
	addImport(file, "kcpreflector", "github.com/kcp-dev/apimachinery/v2/third_party/reflector")
	addImport(file, "", "github.com/kcp-dev/logicalcluster/v3")
	addImport(file, "", "k8s.io/client-go/tools/cache")

	// 4. Qualify bare identifiers with cache. prefix.
	localDefs := toSet(
		// Types/structs defined locally in the output file:
		"HandlerOptions",
		"InformerSynced",
		"sharedIndexInformer",
		"dummyController",
		"updateNotification", "addNotification", "deleteNotification",
		"sharedProcessor", "processorListener",
	)
	qualifyIdents(file, "cache", cacheTypesToQualify, localDefs)

	// 5. Render AST to source.
	out, err := renderFile(fset, file)
	if err != nil {
		return nil, err
	}

	// 6. Targeted text replacements for kcp-specific changes.

	// 6a. Change return types of constructors.
	out = replaceInSource(out, "func NewSharedIndexInformer(lw cache.ListerWatcher, exampleObject runtime.Object, defaultEventHandlerResyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {",
		"func NewSharedIndexInformer(lw cache.ListerWatcher, exampleObject runtime.Object, defaultEventHandlerResyncPeriod time.Duration, indexers cache.Indexers) kcpcache.ScopeableSharedIndexInformer {")
	out = replaceInSource(out, "func NewSharedIndexInformerWithOptions(lw cache.ListerWatcher, exampleObject runtime.Object, options cache.SharedIndexInformerOptions) cache.SharedIndexInformer {",
		"func NewSharedIndexInformerWithOptions(lw cache.ListerWatcher, exampleObject runtime.Object, options cache.SharedIndexInformerOptions) kcpcache.ScopeableSharedIndexInformer {")
	out = replaceInSource(out, "func NewSharedInformer(lw cache.ListerWatcher, exampleObject runtime.Object, defaultEventHandlerResyncPeriod time.Duration) cache.SharedInformer {",
		"func NewSharedInformer(lw cache.ListerWatcher, exampleObject runtime.Object, defaultEventHandlerResyncPeriod time.Duration) cache.SharedInformer {")

	// 6b. Replace key function in NewIndexer call.
	out = replaceInSource(out,
		"cache.NewIndexer(cache.DeletionHandlingMetaNamespaceKeyFunc, options.Indexers)",
		"// kcp modification: cluster-aware key function\n\t\tcache.NewIndexer(kcpcache.MetaClusterNamespaceKeyFunc, options.Indexers)")

	// 6c. Replace newQueueFIFO call in RunWithContext with inline FIFO creation.
	out = replaceInSource(out,
		"fifo := newQueueFIFO(s.indexer, s.transform)",
		`var fifo cache.Queue
		if clientgofeaturegate.FeatureGates().Enabled(clientgofeaturegate.InOrderInformers) {
			fifo = cache.NewRealFIFOWithOptions(cache.RealFIFOOptions{
				// kcp modification: cluster-aware key function
				KeyFunction:  kcpcache.MetaClusterNamespaceKeyFunc,
				KnownObjects: s.indexer,
				Transformer:  s.transform,
			})
		} else {
			fifo = cache.NewDeltaFIFOWithOptions(cache.DeltaFIFOOptions{
				KnownObjects:          s.indexer,
				EmitDeltaTypeReplaced: true,
				Transformer:           s.transform,
				// kcp modification: cluster-aware key function
				KeyFunction: kcpcache.MetaClusterNamespaceKeyFunc,
			})
		}`)

	// 6d. Replace Config type and controller creation.
	out = replaceInSource(out, "cfg := &cache.Config{", `// kcp modification: use forked controller with cluster-aware key function support
		cfg := &kcpreflector.Config{`)

	// 6e. Add KeyFunction field to config.
	out = replaceInSource(out,
		"WatchErrorHandlerWithContext: s.watchErrorHandler,\n\t\t}",
		"WatchErrorHandlerWithContext: s.watchErrorHandler,\n\t\t\tKeyFunction:                  kcpcache.DeletionHandlingMetaClusterNamespaceKeyFunc,\n\t\t}")

	// 6f. Replace controller creation.
	out = replaceInSource(out, "s.controller = cache.New(cfg)", "s.controller = kcpreflector.New(cfg)")

	// 6g. Remove the s.controller.(*controller).clock line.
	out = replaceInSource(out,
		"s.controller.(*controller).clock = s.clock\n",
		"// kcp modification: removed s.controller.(*controller).clock - unexported field\n")

	// 6h. Fix LastSyncResourceVersion in dummyController to use .controller.
	out = replaceInSource(out,
		"return v.informer.LastSyncResourceVersion()",
		"return v.informer.controller.LastSyncResourceVersion()")

	// 6i. Add Cluster/ClusterWithContext methods after sharedIndexInformer struct.
	clusterMethods := `

func (s *sharedIndexInformer) Cluster(cluster logicalcluster.Name) cache.SharedIndexInformer {
	return newScopedSharedIndexInformer(s, cluster)
}

func (s *sharedIndexInformer) ClusterWithContext(ctx context.Context, cluster logicalcluster.Name) cache.SharedIndexInformer {
	return newScopedSharedIndexInformerWithContext(ctx, s, cluster)
}
`
	out = insertAfterLine(out, "transform cache.TransformFunc", clusterMethods)

	// 6j. Extract and append processDeltas and processDeltasInBatch from upstream controller.go.
	pdSrc, err := extractFunc(upstreamControllerFset, upstreamControllerAST, "processDeltas")
	if err != nil {
		return nil, fmt.Errorf("extracting processDeltas: %w", err)
	}
	pdbSrc, err := extractFunc(upstreamControllerFset, upstreamControllerAST, "processDeltasInBatch")
	if err != nil {
		return nil, fmt.Errorf("extracting processDeltasInBatch: %w", err)
	}

	// Qualify the extracted functions.
	pdSrc = qualifyExtractedFunc(pdSrc)
	pdbSrc = qualifyExtractedFunc(pdbSrc)

	appendix := "\n// kcp modification: copied from controller.go (unexported upstream)\n" + pdSrc + "\n\n" + pdbSrc + "\n"
	out = append(out, []byte(appendix)...)

	// 7. Add copyright modification.
	out = addCopyrightModification(out)

	// 8. Final gofmt.
	out, err = formatSource(out)
	if err != nil {
		return nil, fmt.Errorf("final format: %w", err)
	}

	return out, nil
}

// qualifyExtractedFunc applies cache. qualifications to a rendered function string
// extracted from the upstream controller.go.
func qualifyExtractedFunc(src string) string {
	replacements := [][2]string{
		{"ResourceEventHandler,", "cache.ResourceEventHandler,"},
		{"\tStore,", "\tcache.Store,"},
		{"clientState Store", "clientState cache.Store"},
		{"deltas Deltas", "deltas cache.Deltas"},
		{"deltas []Delta", "deltas []cache.Delta"},
		{"case Sync, Replaced, Added, Updated:", "case cache.Sync, cache.Replaced, cache.Added, cache.Updated:"},
		{"case Deleted:", "case cache.Deleted:"},
		{"TransactionStore", "cache.TransactionStore"},
		{"Transaction{", "cache.Transaction{"},
		{"TransactionTypeUpdate", "cache.TransactionTypeUpdate"},
		{"TransactionTypeAdd", "cache.TransactionTypeAdd"},
		{"TransactionTypeDelete", "cache.TransactionTypeDelete"},
		{"handler ResourceEventHandler", "handler cache.ResourceEventHandler"},
		{"clientState Store", "clientState cache.Store"},
		{"Deltas{delta}", "cache.Deltas{delta}"},
	}
	for _, r := range replacements {
		src = replaceAll(src, r[0], r[1])
	}
	return src
}

func replaceAll(s, old, new string) string {
	for {
		i := indexOf(s, old)
		if i < 0 {
			return s
		}
		s = s[:i] + new + s[i+len(old):]
	}
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func formatSource(src []byte) ([]byte, error) {
	formatted, err := format.Source(src)
	if err != nil {
		return src, err
	}
	return formatted, nil
}
