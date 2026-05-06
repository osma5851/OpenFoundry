package stores_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openfoundry/openfoundry-go/libs/ontology-kernel/stores"
	storageabstraction "github.com/openfoundry/openfoundry-go/libs/storage-abstraction"
)

// libs/ontology-kernel/src/stores/mod.rs `Stores::in_memory()` —
// the constructor returns a fully-wired bag where every trait field
// is satisfied.
func TestNewInMemoryWiresAllStores(t *testing.T) {
	s := stores.NewInMemory()
	require.NotNil(t, s.Objects)
	require.NotNil(t, s.Links)
	require.NotNil(t, s.Actions)

	// Compile-time pin doubled at runtime via interface assertion.
	var _ storageabstraction.ObjectStore = s.Objects
	var _ storageabstraction.LinkStore = s.Links
	var _ storageabstraction.ActionLogStore = s.Actions
}

// libs/ontology-kernel/src/stores/pg.rs — every PostgresObjectStore
// method returns the verbatim NOT_YET error wrapped as
// RepoError::Backend. Mirrors the Rust contract for the stub
// adapters used while a service has not migrated off direct PG.
func TestPostgresObjectStoreReturnsNotYetError(t *testing.T) {
	s := &stores.PostgresObjectStore{Pool: nil}
	ctx := context.Background()

	const wantSubstr = "PostgreSQL adapter for storage-abstraction trait is a stub"

	_, err := s.Get(ctx, "tenant-1", "obj-1", storageabstraction.Strong())
	require.Error(t, err)
	assert.Contains(t, err.Error(), wantSubstr)
	assert.True(t, storageabstraction.IsBackendError(err))

	_, err = s.Put(ctx, storageabstraction.Object{}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), wantSubstr)

	_, err = s.Delete(ctx, "tenant-1", "obj-1")
	require.Error(t, err)

	_, err = s.ListByType(ctx, "tenant-1", "type", storageabstraction.Page{}, storageabstraction.Strong())
	require.Error(t, err)
	_, err = s.ListByOwner(ctx, "tenant-1", "owner", storageabstraction.Page{}, storageabstraction.Strong())
	require.Error(t, err)
	_, err = s.ListByMarking(ctx, "tenant-1", "marking", storageabstraction.Page{}, storageabstraction.Strong())
	require.Error(t, err)
}

// libs/ontology-kernel/src/stores/pg.rs — same NOT_YET contract for
// the Link and ActionLog adapters.
func TestPostgresLinkAndActionLogReturnNotYet(t *testing.T) {
	ctx := context.Background()
	link := &stores.PostgresLinkStore{}
	action := &stores.PostgresActionLogStore{}

	require.Error(t, link.Put(ctx, storageabstraction.Link{}))
	_, err := link.Delete(ctx, "t", "lt", "f", "to")
	require.Error(t, err)

	require.Error(t, action.Append(ctx, storageabstraction.ActionLogEntry{}))
}

// libs/ontology-kernel/src/stores/mod.rs `Stores::in_memory()` —
// the in-memory ObjectStore round-trips a put/get and applies
// optimistic concurrency the same way the production stores do.
func TestInMemoryObjectStoreRoundTripAndConflict(t *testing.T) {
	ctx := context.Background()
	s := stores.NewInMemoryObjectStore()

	obj := storageabstraction.Object{
		Tenant:  "t",
		ID:      "o",
		TypeID:  "ty",
		Version: 1,
	}
	out, err := s.Put(ctx, obj, nil)
	require.NoError(t, err)
	assert.Equal(t, storageabstraction.PutInserted, out.Kind)

	// Insert-only of an existing key conflicts.
	out, err = s.Put(ctx, obj, nil)
	require.NoError(t, err)
	assert.Equal(t, storageabstraction.PutVersionConflict, out.Kind)

	// Update with matching expected_version succeeds.
	v := uint64(1)
	obj.Version = 2
	out, err = s.Put(ctx, obj, &v)
	require.NoError(t, err)
	assert.Equal(t, storageabstraction.PutUpdated, out.Kind)
	assert.Equal(t, uint64(1), out.PreviousVersion)
	assert.Equal(t, uint64(2), out.NewVersion)

	// Update with stale expected_version conflicts.
	stale := uint64(1)
	obj.Version = 3
	out, err = s.Put(ctx, obj, &stale)
	require.NoError(t, err)
	assert.Equal(t, storageabstraction.PutVersionConflict, out.Kind)
	assert.Equal(t, uint64(1), out.ExpectedVersion)
	assert.Equal(t, uint64(2), out.ActualVersion)

	// Get returns the latest committed copy.
	got, err := s.Get(ctx, "t", "o", storageabstraction.Strong())
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, uint64(2), got.Version)

	// Delete is idempotent.
	ok, err := s.Delete(ctx, "t", "o")
	require.NoError(t, err)
	assert.True(t, ok)
	ok, err = s.Delete(ctx, "t", "o")
	require.NoError(t, err)
	assert.False(t, ok)
}

// libs/ontology-kernel/src/stores/mod.rs InMemoryLinkStore — Put is
// idempotent on the (tenant, link_type, from, to) triple.
func TestInMemoryLinkStoreIdempotentPut(t *testing.T) {
	ctx := context.Background()
	s := stores.NewInMemoryLinkStore()

	link := storageabstraction.Link{
		Tenant:   "t",
		LinkType: "owns",
		From:     "a",
		To:       "b",
	}
	require.NoError(t, s.Put(ctx, link))
	require.NoError(t, s.Put(ctx, link)) // second Put is a no-op.

	out, err := s.ListOutgoing(ctx, "t", "owns", "a", storageabstraction.Page{}, storageabstraction.Strong())
	require.NoError(t, err)
	assert.Len(t, out.Items, 1)

	in, err := s.ListIncoming(ctx, "t", "owns", "b", storageabstraction.Page{}, storageabstraction.Strong())
	require.NoError(t, err)
	assert.Len(t, in.Items, 1)

	// Delete returns false on second call.
	ok, err := s.Delete(ctx, "t", "owns", "a", "b")
	require.NoError(t, err)
	assert.True(t, ok)
	ok, err = s.Delete(ctx, "t", "owns", "a", "b")
	require.NoError(t, err)
	assert.False(t, ok)
}

// libs/ontology-kernel/src/stores/mod.rs InMemoryActionLogStore —
// ListRecent / ListForObject / ListForAction return time-DESC and
// honour tenant scoping.
func TestInMemoryActionLogScopingAndOrdering(t *testing.T) {
	ctx := context.Background()
	s := stores.NewInMemoryActionLogStore()
	objA := storageabstraction.ObjectId("A")
	mk := func(tenant, action string, obj *storageabstraction.ObjectId, recordedAt int64) storageabstraction.ActionLogEntry {
		return storageabstraction.ActionLogEntry{
			Tenant:       storageabstraction.TenantId(tenant),
			ActionID:     action,
			Object:       obj,
			RecordedAtMs: recordedAt,
		}
	}
	require.NoError(t, s.Append(ctx, mk("t", "act-1", &objA, 1)))
	require.NoError(t, s.Append(ctx, mk("t", "act-2", nil, 2)))
	require.NoError(t, s.Append(ctx, mk("other", "act-3", &objA, 3)))

	recent, err := s.ListRecent(ctx, "t", storageabstraction.Page{}, storageabstraction.Strong())
	require.NoError(t, err)
	require.Len(t, recent.Items, 2)
	// Newest-first: act-2 (recorded_at=2) precedes act-1 (recorded_at=1).
	assert.Equal(t, "act-2", recent.Items[0].ActionID)
	assert.Equal(t, "act-1", recent.Items[1].ActionID)

	forObj, err := s.ListForObject(ctx, "t", objA, storageabstraction.Page{}, storageabstraction.Strong())
	require.NoError(t, err)
	require.Len(t, forObj.Items, 1)
	assert.Equal(t, "act-1", forObj.Items[0].ActionID)

	forAct, err := s.ListForAction(ctx, "t", "act-1", storageabstraction.Page{}, storageabstraction.Strong())
	require.NoError(t, err)
	require.Len(t, forAct.Items, 1)

	// Other tenant is invisible.
	otherTenant, err := s.ListRecent(ctx, "other", storageabstraction.Page{}, storageabstraction.Strong())
	require.NoError(t, err)
	require.Len(t, otherTenant.Items, 1)
	assert.Equal(t, "act-3", otherTenant.Items[0].ActionID)
}

// libs/ontology-kernel/src/stores/mock.rs `mockall::mock!`
// equivalent — record-and-return surfaces queue responses and
// expose every call for assertion.
func TestMockObjectStoreRecordsAndReturns(t *testing.T) {
	ctx := context.Background()
	m := stores.NewMockObjectStore()
	m.QueueGet(&storageabstraction.Object{ID: "o", Version: 7}, nil)
	m.QueuePut(storageabstraction.Updated(6, 7), nil)

	got, err := m.Get(ctx, "t", "o", storageabstraction.Strong())
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, uint64(7), got.Version)
	assert.Len(t, m.GetCalls, 1)
	assert.Equal(t, storageabstraction.TenantId("t"), m.GetCalls[0].Tenant)

	out, err := m.Put(ctx, storageabstraction.Object{ID: "o", Version: 7}, nil)
	require.NoError(t, err)
	assert.Equal(t, storageabstraction.PutUpdated, out.Kind)
	assert.Len(t, m.PutCalls, 1)

	// No queued response → zero values, but the call is still recorded.
	_, err = m.Delete(ctx, "t", "o")
	require.NoError(t, err)
	assert.Len(t, m.DeleteCalls, 1)
}

// libs/ontology-kernel/src/stores/mock.rs MockLinkStore +
// MockActionLogStore satisfy their respective trait surfaces.
func TestMockLinkAndActionStoresImplementInterfaces(t *testing.T) {
	var _ storageabstraction.LinkStore = stores.NewMockLinkStore()
	var _ storageabstraction.ActionLogStore = stores.NewMockActionLogStore()
}
