// Package stores is the Go port of `libs/ontology-kernel/src/stores/*`.
//
// All persistence in ontology-kernel is being migrated from raw `pgx`
// call sites to the storage-abstraction interfaces so that the same
// handlers can be wired against:
//
//   - CassandraObjectStore / CassandraLinkStore / CassandraActionLogStore
//     (the production target — see ADR-0020),
//   - the legacy Postgres adapters in [pg.go] (only behind the
//     legacy-pg build tag, used while handlers are migrated one
//     service at a time per
//     docs/architecture/migration-plan-cassandra-foundry-parity.md
//     §S1.4–S1.7),
//   - hand-rolled fakes in [mock.go] for unit tests.
//
// The kernel's [github.com/openfoundry/openfoundry-go/libs/ontology-kernel.AppState]
// carries a single [Stores] handle so handlers stay infrastructure-
// agnostic.
//
// Coverage gap vs Rust: the Rust source declares seven trait fields —
// objects / links / actions / definitions / read_models / search /
// object_set_materializations. The Go storage-abstraction package
// currently exposes the first three (plus SchemaStore + SessionStore
// that the Rust crate does not use here). The remaining four trait
// surfaces (DefinitionStore, ReadModelStore, SearchBackend,
// ObjectSetMaterializationStore) will land alongside the relevant
// domain-layer iters that actually consume them; until then [Stores]
// only models the three production stores.
package stores

import storageabstraction "github.com/openfoundry/openfoundry-go/libs/storage-abstraction"

// Stores mirrors `pub struct Stores` in src/stores/mod.rs — the
// trait-object bag that ontology-kernel handlers route their I/O
// through.
type Stores struct {
	Objects storageabstraction.ObjectStore
	Links   storageabstraction.LinkStore
	Actions storageabstraction.ActionLogStore
}

// NewInMemory mirrors `Stores::in_memory()`. Returns a Stores backed
// by hand-rolled in-process fakes from [mock.go]. Intended for unit
// tests and for smoke-testing handlers without spinning up
// infrastructure.
func NewInMemory() Stores {
	return Stores{
		Objects: NewInMemoryObjectStore(),
		Links:   NewInMemoryLinkStore(),
		Actions: NewInMemoryActionLogStore(),
	}
}
