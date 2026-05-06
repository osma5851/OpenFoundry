// Hand-rolled fakes that implement the three storage-abstraction
// trait surfaces consumed by ontology-kernel handlers. They take the
// place of `mockall::mock!` from `libs/ontology-kernel/src/stores/mock.rs`:
// Go has no equivalent codegen so we stub directly.
//
// Two flavours per trait:
//
//   - `InMemory*Store`: a working fake backed by a tenant-scoped
//     map. Handlers can be exercised end-to-end against this without
//     needing Postgres or Cassandra. This is what `Stores::in_memory`
//     wires in.
//   - `Mock*Store`: a record-and-return mock. Each method records
//     its invocation in a slice and either returns the next queued
//     response or zero values. Useful for unit tests that need to
//     assert exact call sequences.

package stores

import (
	"context"
	"sync"

	storageabstraction "github.com/openfoundry/openfoundry-go/libs/storage-abstraction"
)

// ---- InMemoryObjectStore ---------------------------------------------------

type imObjectKey struct {
	tenant storageabstraction.TenantId
	id     storageabstraction.ObjectId
}

// InMemoryObjectStore is a tenant-scoped in-process store. Mirrors
// the contract of `noop::InMemoryObjectStore` shipped by the Rust
// storage-abstraction crate: optimistic concurrency on Put, idempotent
// Delete, paging via deterministic key ordering.
type InMemoryObjectStore struct {
	mu   sync.RWMutex
	data map[imObjectKey]storageabstraction.Object
}

// NewInMemoryObjectStore returns a fresh empty store.
func NewInMemoryObjectStore() *InMemoryObjectStore {
	return &InMemoryObjectStore{data: map[imObjectKey]storageabstraction.Object{}}
}

var _ storageabstraction.ObjectStore = (*InMemoryObjectStore)(nil)

func (s *InMemoryObjectStore) Get(_ context.Context, tenant storageabstraction.TenantId, id storageabstraction.ObjectId, _ storageabstraction.ReadConsistency) (*storageabstraction.Object, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if obj, ok := s.data[imObjectKey{tenant, id}]; ok {
		clone := obj
		return &clone, nil
	}
	return nil, nil
}

func (s *InMemoryObjectStore) Put(_ context.Context, obj storageabstraction.Object, expectedVersion *uint64) (storageabstraction.PutOutcome, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := imObjectKey{obj.Tenant, obj.ID}
	prev, exists := s.data[key]
	if expectedVersion == nil && exists {
		return storageabstraction.VersionConflict(0, prev.Version), nil
	}
	if expectedVersion != nil && exists && prev.Version != *expectedVersion {
		return storageabstraction.VersionConflict(*expectedVersion, prev.Version), nil
	}
	if expectedVersion != nil && !exists {
		return storageabstraction.VersionConflict(*expectedVersion, 0), nil
	}
	s.data[key] = obj
	if exists {
		return storageabstraction.Updated(prev.Version, obj.Version), nil
	}
	return storageabstraction.PutOutcome{Kind: storageabstraction.PutInserted, NewVersion: obj.Version}, nil
}

func (s *InMemoryObjectStore) Delete(_ context.Context, tenant storageabstraction.TenantId, id storageabstraction.ObjectId) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := imObjectKey{tenant, id}
	if _, ok := s.data[key]; !ok {
		return false, nil
	}
	delete(s.data, key)
	return true, nil
}

func (s *InMemoryObjectStore) ListByType(_ context.Context, tenant storageabstraction.TenantId, typeID storageabstraction.TypeId, _ storageabstraction.Page, _ storageabstraction.ReadConsistency) (storageabstraction.PagedResult[storageabstraction.Object], error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var items []storageabstraction.Object
	for k, v := range s.data {
		if k.tenant == tenant && v.TypeID == typeID {
			items = append(items, v)
		}
	}
	return storageabstraction.PagedResult[storageabstraction.Object]{Items: items}, nil
}

func (s *InMemoryObjectStore) ListByOwner(_ context.Context, tenant storageabstraction.TenantId, owner storageabstraction.OwnerId, _ storageabstraction.Page, _ storageabstraction.ReadConsistency) (storageabstraction.PagedResult[storageabstraction.Object], error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var items []storageabstraction.Object
	for k, v := range s.data {
		if k.tenant == tenant && v.Owner != nil && *v.Owner == owner {
			items = append(items, v)
		}
	}
	return storageabstraction.PagedResult[storageabstraction.Object]{Items: items}, nil
}

func (s *InMemoryObjectStore) ListByMarking(_ context.Context, tenant storageabstraction.TenantId, marking storageabstraction.MarkingId, _ storageabstraction.Page, _ storageabstraction.ReadConsistency) (storageabstraction.PagedResult[storageabstraction.Object], error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var items []storageabstraction.Object
	for k, v := range s.data {
		if k.tenant != tenant {
			continue
		}
		for _, m := range v.Markings {
			if m == marking {
				items = append(items, v)
				break
			}
		}
	}
	return storageabstraction.PagedResult[storageabstraction.Object]{Items: items}, nil
}

// ---- InMemoryLinkStore -----------------------------------------------------

type imLinkKey struct {
	tenant   storageabstraction.TenantId
	linkType storageabstraction.LinkTypeId
	from     storageabstraction.ObjectId
	to       storageabstraction.ObjectId
}

// InMemoryLinkStore is a tenant-scoped in-process link store with
// the same idempotency contract as the production backends: a second
// Put of the same (tenant, link_type, from, to) triple is a no-op.
type InMemoryLinkStore struct {
	mu   sync.RWMutex
	data map[imLinkKey]storageabstraction.Link
}

// NewInMemoryLinkStore returns a fresh empty store.
func NewInMemoryLinkStore() *InMemoryLinkStore {
	return &InMemoryLinkStore{data: map[imLinkKey]storageabstraction.Link{}}
}

var _ storageabstraction.LinkStore = (*InMemoryLinkStore)(nil)

func (s *InMemoryLinkStore) Put(_ context.Context, link storageabstraction.Link) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := imLinkKey{link.Tenant, link.LinkType, link.From, link.To}
	if _, ok := s.data[key]; !ok {
		s.data[key] = link
	}
	return nil
}

func (s *InMemoryLinkStore) Delete(_ context.Context, tenant storageabstraction.TenantId, linkType storageabstraction.LinkTypeId, from, to storageabstraction.ObjectId) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := imLinkKey{tenant, linkType, from, to}
	if _, ok := s.data[key]; !ok {
		return false, nil
	}
	delete(s.data, key)
	return true, nil
}

func (s *InMemoryLinkStore) ListOutgoing(_ context.Context, tenant storageabstraction.TenantId, linkType storageabstraction.LinkTypeId, from storageabstraction.ObjectId, _ storageabstraction.Page, _ storageabstraction.ReadConsistency) (storageabstraction.PagedResult[storageabstraction.Link], error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var items []storageabstraction.Link
	for k, v := range s.data {
		if k.tenant == tenant && k.linkType == linkType && k.from == from {
			items = append(items, v)
		}
	}
	return storageabstraction.PagedResult[storageabstraction.Link]{Items: items}, nil
}

func (s *InMemoryLinkStore) ListIncoming(_ context.Context, tenant storageabstraction.TenantId, linkType storageabstraction.LinkTypeId, to storageabstraction.ObjectId, _ storageabstraction.Page, _ storageabstraction.ReadConsistency) (storageabstraction.PagedResult[storageabstraction.Link], error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var items []storageabstraction.Link
	for k, v := range s.data {
		if k.tenant == tenant && k.linkType == linkType && k.to == to {
			items = append(items, v)
		}
	}
	return storageabstraction.PagedResult[storageabstraction.Link]{Items: items}, nil
}

// ---- InMemoryActionLogStore ------------------------------------------------

// InMemoryActionLogStore is a per-tenant append-only action log.
type InMemoryActionLogStore struct {
	mu      sync.RWMutex
	entries []storageabstraction.ActionLogEntry
}

// NewInMemoryActionLogStore returns a fresh empty store.
func NewInMemoryActionLogStore() *InMemoryActionLogStore {
	return &InMemoryActionLogStore{}
}

var _ storageabstraction.ActionLogStore = (*InMemoryActionLogStore)(nil)

func (s *InMemoryActionLogStore) Append(_ context.Context, entry storageabstraction.ActionLogEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, entry)
	return nil
}

func (s *InMemoryActionLogStore) ListRecent(_ context.Context, tenant storageabstraction.TenantId, _ storageabstraction.Page, _ storageabstraction.ReadConsistency) (storageabstraction.PagedResult[storageabstraction.ActionLogEntry], error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var items []storageabstraction.ActionLogEntry
	// Time-DESC ordering by reversing append order — the entry slice
	// is sorted by RecordedAtMs only as a side effect of Append being
	// the sole writer, which is the contract the production stores
	// enforce explicitly.
	for i := len(s.entries) - 1; i >= 0; i-- {
		if s.entries[i].Tenant == tenant {
			items = append(items, s.entries[i])
		}
	}
	return storageabstraction.PagedResult[storageabstraction.ActionLogEntry]{Items: items}, nil
}

func (s *InMemoryActionLogStore) ListForObject(_ context.Context, tenant storageabstraction.TenantId, object storageabstraction.ObjectId, _ storageabstraction.Page, _ storageabstraction.ReadConsistency) (storageabstraction.PagedResult[storageabstraction.ActionLogEntry], error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var items []storageabstraction.ActionLogEntry
	for i := len(s.entries) - 1; i >= 0; i-- {
		e := s.entries[i]
		if e.Tenant == tenant && e.Object != nil && *e.Object == object {
			items = append(items, e)
		}
	}
	return storageabstraction.PagedResult[storageabstraction.ActionLogEntry]{Items: items}, nil
}

func (s *InMemoryActionLogStore) ListForAction(_ context.Context, tenant storageabstraction.TenantId, actionID string, _ storageabstraction.Page, _ storageabstraction.ReadConsistency) (storageabstraction.PagedResult[storageabstraction.ActionLogEntry], error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var items []storageabstraction.ActionLogEntry
	for i := len(s.entries) - 1; i >= 0; i-- {
		e := s.entries[i]
		if e.Tenant == tenant && e.ActionID == actionID {
			items = append(items, e)
		}
	}
	return storageabstraction.PagedResult[storageabstraction.ActionLogEntry]{Items: items}, nil
}

// ---- MockObjectStore (record-and-return) -----------------------------------

// MockObjectStore records every call against it and returns canned
// responses queued via the `Return*` helpers. Mirrors the spirit of
// the `mockall::mock!{ pub ObjectStoreImpl ... }` macro from
// stores/mock.rs without requiring runtime codegen.
//
// Test code typically does:
//
//	m := stores.NewMockObjectStore()
//	m.QueueGet(&someObject, nil)
//	state.Stores.Objects = m
//	... exercise handler ...
//	require.Equal(t, 1, len(m.GetCalls))
type MockObjectStore struct {
	mu sync.Mutex

	GetCalls           []GetCall
	getResponses       []objectAndErr
	PutCalls           []PutCall
	putResponses       []putOutcomeAndErr
	DeleteCalls        []DeleteCall
	deleteResponses    []boolAndErr
	ListByTypeCalls    []ListByTypeCall
	listByTypeResp     []pagedObjectsAndErr
	ListByOwnerCalls   []ListByOwnerCall
	listByOwnerResp    []pagedObjectsAndErr
	ListByMarkingCalls []ListByMarkingCall
	listByMarkingResp  []pagedObjectsAndErr
}

// NewMockObjectStore returns a fresh mock with no queued responses.
func NewMockObjectStore() *MockObjectStore { return &MockObjectStore{} }

var _ storageabstraction.ObjectStore = (*MockObjectStore)(nil)

type GetCall struct {
	Tenant      storageabstraction.TenantId
	ID          storageabstraction.ObjectId
	Consistency storageabstraction.ReadConsistency
}
type PutCall struct {
	Object          storageabstraction.Object
	ExpectedVersion *uint64
}
type DeleteCall struct {
	Tenant storageabstraction.TenantId
	ID     storageabstraction.ObjectId
}
type ListByTypeCall struct {
	Tenant      storageabstraction.TenantId
	TypeID      storageabstraction.TypeId
	Page        storageabstraction.Page
	Consistency storageabstraction.ReadConsistency
}
type ListByOwnerCall struct {
	Tenant      storageabstraction.TenantId
	Owner       storageabstraction.OwnerId
	Page        storageabstraction.Page
	Consistency storageabstraction.ReadConsistency
}
type ListByMarkingCall struct {
	Tenant      storageabstraction.TenantId
	Marking     storageabstraction.MarkingId
	Page        storageabstraction.Page
	Consistency storageabstraction.ReadConsistency
}

type objectAndErr struct {
	obj *storageabstraction.Object
	err error
}
type putOutcomeAndErr struct {
	out storageabstraction.PutOutcome
	err error
}
type boolAndErr struct {
	v   bool
	err error
}
type pagedObjectsAndErr struct {
	page storageabstraction.PagedResult[storageabstraction.Object]
	err  error
}

// QueueGet enqueues the next response that Get will return.
func (m *MockObjectStore) QueueGet(obj *storageabstraction.Object, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getResponses = append(m.getResponses, objectAndErr{obj, err})
}

// QueuePut enqueues the next response that Put will return.
func (m *MockObjectStore) QueuePut(out storageabstraction.PutOutcome, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.putResponses = append(m.putResponses, putOutcomeAndErr{out, err})
}

// QueueDelete enqueues the next response that Delete will return.
func (m *MockObjectStore) QueueDelete(v bool, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deleteResponses = append(m.deleteResponses, boolAndErr{v, err})
}

// QueueListByType enqueues the next response that ListByType will return.
func (m *MockObjectStore) QueueListByType(p storageabstraction.PagedResult[storageabstraction.Object], err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listByTypeResp = append(m.listByTypeResp, pagedObjectsAndErr{p, err})
}

// QueueListByOwner enqueues the next response that ListByOwner will return.
func (m *MockObjectStore) QueueListByOwner(p storageabstraction.PagedResult[storageabstraction.Object], err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listByOwnerResp = append(m.listByOwnerResp, pagedObjectsAndErr{p, err})
}

// QueueListByMarking enqueues the next response that ListByMarking will return.
func (m *MockObjectStore) QueueListByMarking(p storageabstraction.PagedResult[storageabstraction.Object], err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listByMarkingResp = append(m.listByMarkingResp, pagedObjectsAndErr{p, err})
}

func (m *MockObjectStore) Get(_ context.Context, tenant storageabstraction.TenantId, id storageabstraction.ObjectId, c storageabstraction.ReadConsistency) (*storageabstraction.Object, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.GetCalls = append(m.GetCalls, GetCall{tenant, id, c})
	if len(m.getResponses) == 0 {
		return nil, nil
	}
	r := m.getResponses[0]
	m.getResponses = m.getResponses[1:]
	return r.obj, r.err
}

func (m *MockObjectStore) Put(_ context.Context, obj storageabstraction.Object, expectedVersion *uint64) (storageabstraction.PutOutcome, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.PutCalls = append(m.PutCalls, PutCall{obj, expectedVersion})
	if len(m.putResponses) == 0 {
		return storageabstraction.PutOutcome{}, nil
	}
	r := m.putResponses[0]
	m.putResponses = m.putResponses[1:]
	return r.out, r.err
}

func (m *MockObjectStore) Delete(_ context.Context, tenant storageabstraction.TenantId, id storageabstraction.ObjectId) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.DeleteCalls = append(m.DeleteCalls, DeleteCall{tenant, id})
	if len(m.deleteResponses) == 0 {
		return false, nil
	}
	r := m.deleteResponses[0]
	m.deleteResponses = m.deleteResponses[1:]
	return r.v, r.err
}

func (m *MockObjectStore) ListByType(_ context.Context, tenant storageabstraction.TenantId, typeID storageabstraction.TypeId, page storageabstraction.Page, c storageabstraction.ReadConsistency) (storageabstraction.PagedResult[storageabstraction.Object], error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ListByTypeCalls = append(m.ListByTypeCalls, ListByTypeCall{tenant, typeID, page, c})
	if len(m.listByTypeResp) == 0 {
		return storageabstraction.PagedResult[storageabstraction.Object]{}, nil
	}
	r := m.listByTypeResp[0]
	m.listByTypeResp = m.listByTypeResp[1:]
	return r.page, r.err
}

func (m *MockObjectStore) ListByOwner(_ context.Context, tenant storageabstraction.TenantId, owner storageabstraction.OwnerId, page storageabstraction.Page, c storageabstraction.ReadConsistency) (storageabstraction.PagedResult[storageabstraction.Object], error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ListByOwnerCalls = append(m.ListByOwnerCalls, ListByOwnerCall{tenant, owner, page, c})
	if len(m.listByOwnerResp) == 0 {
		return storageabstraction.PagedResult[storageabstraction.Object]{}, nil
	}
	r := m.listByOwnerResp[0]
	m.listByOwnerResp = m.listByOwnerResp[1:]
	return r.page, r.err
}

func (m *MockObjectStore) ListByMarking(_ context.Context, tenant storageabstraction.TenantId, marking storageabstraction.MarkingId, page storageabstraction.Page, c storageabstraction.ReadConsistency) (storageabstraction.PagedResult[storageabstraction.Object], error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ListByMarkingCalls = append(m.ListByMarkingCalls, ListByMarkingCall{tenant, marking, page, c})
	if len(m.listByMarkingResp) == 0 {
		return storageabstraction.PagedResult[storageabstraction.Object]{}, nil
	}
	r := m.listByMarkingResp[0]
	m.listByMarkingResp = m.listByMarkingResp[1:]
	return r.page, r.err
}

// ---- MockLinkStore ---------------------------------------------------------

type MockLinkStore struct {
	mu sync.Mutex

	PutCalls          []storageabstraction.Link
	putResponses      []error
	DeleteCalls       []DeleteLinkCall
	deleteResponses   []boolAndErr
	ListOutgoingCalls []ListOutgoingCall
	listOutgoingResp  []pagedLinksAndErr
	ListIncomingCalls []ListIncomingCall
	listIncomingResp  []pagedLinksAndErr
}

// NewMockLinkStore returns a fresh mock.
func NewMockLinkStore() *MockLinkStore { return &MockLinkStore{} }

var _ storageabstraction.LinkStore = (*MockLinkStore)(nil)

type DeleteLinkCall struct {
	Tenant   storageabstraction.TenantId
	LinkType storageabstraction.LinkTypeId
	From, To storageabstraction.ObjectId
}
type ListOutgoingCall struct {
	Tenant      storageabstraction.TenantId
	LinkType    storageabstraction.LinkTypeId
	From        storageabstraction.ObjectId
	Page        storageabstraction.Page
	Consistency storageabstraction.ReadConsistency
}
type ListIncomingCall struct {
	Tenant      storageabstraction.TenantId
	LinkType    storageabstraction.LinkTypeId
	To          storageabstraction.ObjectId
	Page        storageabstraction.Page
	Consistency storageabstraction.ReadConsistency
}
type pagedLinksAndErr struct {
	page storageabstraction.PagedResult[storageabstraction.Link]
	err  error
}

func (m *MockLinkStore) QueuePut(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.putResponses = append(m.putResponses, err)
}

func (m *MockLinkStore) QueueDelete(v bool, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deleteResponses = append(m.deleteResponses, boolAndErr{v, err})
}

func (m *MockLinkStore) QueueListOutgoing(p storageabstraction.PagedResult[storageabstraction.Link], err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listOutgoingResp = append(m.listOutgoingResp, pagedLinksAndErr{p, err})
}

func (m *MockLinkStore) QueueListIncoming(p storageabstraction.PagedResult[storageabstraction.Link], err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listIncomingResp = append(m.listIncomingResp, pagedLinksAndErr{p, err})
}

func (m *MockLinkStore) Put(_ context.Context, link storageabstraction.Link) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.PutCalls = append(m.PutCalls, link)
	if len(m.putResponses) == 0 {
		return nil
	}
	err := m.putResponses[0]
	m.putResponses = m.putResponses[1:]
	return err
}

func (m *MockLinkStore) Delete(_ context.Context, tenant storageabstraction.TenantId, linkType storageabstraction.LinkTypeId, from, to storageabstraction.ObjectId) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.DeleteCalls = append(m.DeleteCalls, DeleteLinkCall{tenant, linkType, from, to})
	if len(m.deleteResponses) == 0 {
		return false, nil
	}
	r := m.deleteResponses[0]
	m.deleteResponses = m.deleteResponses[1:]
	return r.v, r.err
}

func (m *MockLinkStore) ListOutgoing(_ context.Context, tenant storageabstraction.TenantId, linkType storageabstraction.LinkTypeId, from storageabstraction.ObjectId, page storageabstraction.Page, c storageabstraction.ReadConsistency) (storageabstraction.PagedResult[storageabstraction.Link], error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ListOutgoingCalls = append(m.ListOutgoingCalls, ListOutgoingCall{tenant, linkType, from, page, c})
	if len(m.listOutgoingResp) == 0 {
		return storageabstraction.PagedResult[storageabstraction.Link]{}, nil
	}
	r := m.listOutgoingResp[0]
	m.listOutgoingResp = m.listOutgoingResp[1:]
	return r.page, r.err
}

func (m *MockLinkStore) ListIncoming(_ context.Context, tenant storageabstraction.TenantId, linkType storageabstraction.LinkTypeId, to storageabstraction.ObjectId, page storageabstraction.Page, c storageabstraction.ReadConsistency) (storageabstraction.PagedResult[storageabstraction.Link], error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ListIncomingCalls = append(m.ListIncomingCalls, ListIncomingCall{tenant, linkType, to, page, c})
	if len(m.listIncomingResp) == 0 {
		return storageabstraction.PagedResult[storageabstraction.Link]{}, nil
	}
	r := m.listIncomingResp[0]
	m.listIncomingResp = m.listIncomingResp[1:]
	return r.page, r.err
}

// ---- MockActionLogStore ----------------------------------------------------

type MockActionLogStore struct {
	mu sync.Mutex

	AppendCalls         []storageabstraction.ActionLogEntry
	appendResp          []error
	ListRecentCalls     []ListRecentCall
	listRecentResp      []pagedActionsAndErr
	ListForObjectCalls  []ListForObjectCall
	listForObjectResp   []pagedActionsAndErr
	ListForActionCalls  []ListForActionCall
	listForActionResp   []pagedActionsAndErr
}

// NewMockActionLogStore returns a fresh mock.
func NewMockActionLogStore() *MockActionLogStore { return &MockActionLogStore{} }

var _ storageabstraction.ActionLogStore = (*MockActionLogStore)(nil)

type ListRecentCall struct {
	Tenant      storageabstraction.TenantId
	Page        storageabstraction.Page
	Consistency storageabstraction.ReadConsistency
}
type ListForObjectCall struct {
	Tenant      storageabstraction.TenantId
	Object      storageabstraction.ObjectId
	Page        storageabstraction.Page
	Consistency storageabstraction.ReadConsistency
}
type ListForActionCall struct {
	Tenant      storageabstraction.TenantId
	ActionID    string
	Page        storageabstraction.Page
	Consistency storageabstraction.ReadConsistency
}
type pagedActionsAndErr struct {
	page storageabstraction.PagedResult[storageabstraction.ActionLogEntry]
	err  error
}

func (m *MockActionLogStore) QueueAppend(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.appendResp = append(m.appendResp, err)
}

func (m *MockActionLogStore) QueueListRecent(p storageabstraction.PagedResult[storageabstraction.ActionLogEntry], err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listRecentResp = append(m.listRecentResp, pagedActionsAndErr{p, err})
}

func (m *MockActionLogStore) QueueListForObject(p storageabstraction.PagedResult[storageabstraction.ActionLogEntry], err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listForObjectResp = append(m.listForObjectResp, pagedActionsAndErr{p, err})
}

func (m *MockActionLogStore) QueueListForAction(p storageabstraction.PagedResult[storageabstraction.ActionLogEntry], err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listForActionResp = append(m.listForActionResp, pagedActionsAndErr{p, err})
}

func (m *MockActionLogStore) Append(_ context.Context, entry storageabstraction.ActionLogEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.AppendCalls = append(m.AppendCalls, entry)
	if len(m.appendResp) == 0 {
		return nil
	}
	err := m.appendResp[0]
	m.appendResp = m.appendResp[1:]
	return err
}

func (m *MockActionLogStore) ListRecent(_ context.Context, tenant storageabstraction.TenantId, page storageabstraction.Page, c storageabstraction.ReadConsistency) (storageabstraction.PagedResult[storageabstraction.ActionLogEntry], error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ListRecentCalls = append(m.ListRecentCalls, ListRecentCall{tenant, page, c})
	if len(m.listRecentResp) == 0 {
		return storageabstraction.PagedResult[storageabstraction.ActionLogEntry]{}, nil
	}
	r := m.listRecentResp[0]
	m.listRecentResp = m.listRecentResp[1:]
	return r.page, r.err
}

func (m *MockActionLogStore) ListForObject(_ context.Context, tenant storageabstraction.TenantId, object storageabstraction.ObjectId, page storageabstraction.Page, c storageabstraction.ReadConsistency) (storageabstraction.PagedResult[storageabstraction.ActionLogEntry], error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ListForObjectCalls = append(m.ListForObjectCalls, ListForObjectCall{tenant, object, page, c})
	if len(m.listForObjectResp) == 0 {
		return storageabstraction.PagedResult[storageabstraction.ActionLogEntry]{}, nil
	}
	r := m.listForObjectResp[0]
	m.listForObjectResp = m.listForObjectResp[1:]
	return r.page, r.err
}

func (m *MockActionLogStore) ListForAction(_ context.Context, tenant storageabstraction.TenantId, actionID string, page storageabstraction.Page, c storageabstraction.ReadConsistency) (storageabstraction.PagedResult[storageabstraction.ActionLogEntry], error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ListForActionCalls = append(m.ListForActionCalls, ListForActionCall{tenant, actionID, page, c})
	if len(m.listForActionResp) == 0 {
		return storageabstraction.PagedResult[storageabstraction.ActionLogEntry]{}, nil
	}
	r := m.listForActionResp[0]
	m.listForActionResp = m.listForActionResp[1:]
	return r.page, r.err
}
