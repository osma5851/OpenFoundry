# Evaluación de la Ontología de OpenFoundry vs Palantir Foundry / Gotham

> Fecha de la evaluación: 2026-05-08
> Alcance auditado: `libs/ontology-kernel/**`, `services/ontology-*`, `services/object-database-service`, `services/lineage-service`, `services/entity-resolution-service`, `services/iceberg-catalog-service`, `libs/{cassandra-kernel, search-abstraction, storage-abstraction, authz-cedar-go, auth-middleware, audit-trail, outbox, saga, idempotency, vector-store, query-engine, geospatial-core, observability}`, `proto/**`.

---

## 🎯 Puntuación final: **6 / 10**

OpenFoundry implementa un clon **funcional y técnicamente serio** de la ontología de Palantir, con un ~60–65% de cobertura de las capacidades críticas de Foundry/Gotham. Está muy por encima del nivel de un proyecto académico (un universitario raramente pasaría de 2–3), pero queda lejos del producto Palantir, que tiene una década de pulido en polimorfismo, geo-temporalidad, OQL, marking propagation y materializations.

### Desglose por dimensión

| Dimensión                                  | Score   | Comentario clave                                                                                          |
|--------------------------------------------|---------|-----------------------------------------------------------------------------------------------------------|
| Type system & primitivas                   | **6.5** | 14 tipos básicos sólidos pero sin enum/union/genéricos, sin composite PKs, sin status lifecycle           |
| Polimorfismo & composición                 | **5.0** | Interfaces y shared properties OK, pero tipos rígidos: sin herencia, sin polymorphic links, sin link-props |
| Geo & temporal                             | **2.5** | `geo_point` sí, pero geospatial-core vacío, sin polígonos, sin Geotime, sin time-travel real              |
| Validación de datos                        | **6.5** | Validación per-tipo + required + unique, pero sin check constraints, FKs ni cardinality enforcement       |
| Domain modeling                            | **6.5** | Bindings multi-datasource bien, branching/proposals avanzado, falta type hierarchy y semantic types       |
| Query & analytics                          | **5.0** | ObjectSets con traversal+join, pero **sin OQL** y sin álgebra (union/intersect/difference)                 |
| Access control                             | **6.5** | Cedar + markings + RBAC + submission criteria; falta RLS, marking hierarchies, ABAC cableado en handlers  |
| Automation & rules                         | **6.0** | Triggers + effects + machinery queue; pero el `ApplyRuleEffect` es parcial, sin compensation/rollback     |
| Data binding & ETL                         | **6.5** | 3 sync modes, materialization tracking; pero sin transform DSL, sin CDC nativo, sin ingest visual         |
| Actions & operations                       | **6.5** | Form schemas + auth policy + simulación; pero what-if branches son cosméticas, no async, no templates     |
| Branching & versioning                     | **5.5** | Feature branches y proposals existen, pero sin versionado real de ontología ni stable releases            |
| Functions & extensibility                  | **5.0** | Packages + capabilities + metrics; TS ejecuta inline, Python requiere sidecar opcional, sin SDKs oficiales |
| Observability & audit                      | **7.5** | Outbox + Debezium + Iceberg audit + Prometheus + OTLP + action metrics: sólido                            |
| Storage core                               | **7.5** | Cassandra LWT + PG fallback + in-memory + Iceberg; faltan PostGIS, multi-region CDC                       |
| Search                                     | **6.5** | Vespa híbrido (BM25 + ANN) + OpenSearch; falta phonetic, fuzzy, geo, facetting avanzado                   |
| Operaciones distribuidas                   | **8.0** | Outbox + Saga (compensación LIFO) + Idempotency (PG/Cass/mem) + consistency hints                          |
| Vector / semántica                         | **5.0** | Embedding storage + Vespa ANN + cosine in-memory; falta embedding pipeline, reranking, pgvector real      |
| Streaming / CDC                            | **6.0** | Debezium PG + Kafka pubsub; falta Flink/Beam, Kafka Streams DSL, DLQ automático, CDC desde Cassandra      |
| Privacy / DLP                              | **0.5** | Sin differential privacy, sin k-anonymity, sin PII auto-classification, sin redaction                     |
| Lineage                                    | **6.5** | `lineage-service` operativo a nivel dataset/columna; falta lineage de cambios y de markings end-to-end    |

**Promedio ponderado:** ~6.0/10

---

## 📊 Resumen ejecutivo

### Fortalezas reales

1. **Control-plane metadata sólido**: ObjectType, Property, LinkType, Interface, SharedPropertyType, Binding, ActionType, ObjectSet, Rule, Funnel, Quiver, FunctionPackage, Project están modelados con `models/*.go` claros y persistidos en Postgres.
2. **Branching/Proposals**: `OntologyProjectBranch` + `OntologyProjectProposal` con `ConflictResolutions` y `Tasks`/`Comments` — esto es **inusual** en clones de Foundry.
3. **Bindings ricas**: 3 modos de sync (snapshot / incremental / view), branch + version del dataset, default marking, materialization tracking con health.
4. **Action model rico**: `FormSchema` con secciones y overrides condicionales, `AuthorizationPolicy` (clearance, attribute_equals, deny_guest_sessions), simulación via `ActionWhatIfBranch`, plantillas para upload de media.
5. **Submission criteria AST**: 14 operadores, 4 tipos de operandos, evaluador exhaustivo y purísimo en `submission_eval.go`.
6. **Rules + Machinery queue**: triggers tipados, scheduled effects con priority/capability, alertas con severidad, recomendación de cola con scoring.
7. **Object Sets composables**: filtros, traversals (1-4 hops), joins (inner/left), proyecciones, materialization, policies con `restricted_view_id`.
8. **Storage robusto**: Cassandra con LWT optimista (`INSERT IF NOT EXISTS`, `UPDATE IF revision_number=?`), PG fallback, ConsistencyHints, RepoError tipado.
9. **Outbox + Saga + Idempotency**: ADR-0022 cumplido, compensación LIFO, dual-store de idempotencia (PG ON CONFLICT, Cassandra LWT), inyección de OpenLineage headers.
10. **Cedar policy engine** integrado con hot-reload via NATS y audit a Kafka `audit.authz.v1`.
11. **Búsqueda híbrida**: Vespa (lexical BM25 + ANN/HNSW) + OpenSearch + in-memory; phased ranking nativo.
12. **Auditoría end-to-end**: outbox PG → Debezium → Kafka → Iceberg `of_audit.events`.
13. **Lineage operacional** a nivel dataset y columna, con `impact analysis`.
14. **Entity Resolution**: deduplicación, merge strategies, golden records, review queue.
15. **Writeback transaccional** con outbox + version conflict detection y errores tipados (`Primary`, `CommitAfterPrimary`, `OpenTxAfterPrimary`, `VersionConflict`).

### Debilidades críticas (las que más pesan en el score)

1. **Sin OQL** — no hay un lenguaje de consulta declarativo, todo es CRUD + traversal programático.
2. **Sin álgebra de Object Sets** — falta union / intersect / difference / aggregate.
3. **Sin time-travel** — no hay snapshots históricos ni versionado temporal de objetos; sólo `version` monotónico.
4. **Polimorfismo nulo** — `LinkType` exige `SourceTypeID` y `TargetTypeID` exactos; no hay union types ni link-properties.
5. **Property-level markings ausentes** — markings son object-level únicamente; no hay enmascarado por columna.
6. **Sin marking hierarchies / propagación automática** — `MarkingSource::InheritedFromUpstream` está, pero la propagación es manual.
7. **Geospatial vacío** — `geospatial-core` es placeholder; sin PostGIS/H3/S2/Geotime.
8. **Privacy nula** — sin differential privacy, sin k-anonymity, sin DLP/PII auto-classification.
9. **Cardinality y type validation no se enforcen en links** — `composition.go::CreateLink` no valida `cardinality` ni que `from` sea de `source_type_id`.
10. **What-if branches cosméticas** — `ActionWhatIfBranch` se persiste pero no afecta la evaluación real ni hay merge logic.
11. **Python runtime opcional** — `function_runtime.go` retorna `ErrPythonRuntimeNotWired` salvo que se inyecte el sidecar.
12. **Sin SDKs oficiales** generados/publicados (Python/TS/Go) listos para usuarios externos.
13. **Sin streaming subscriptions / WebSockets** para clientes.
14. **`exploratory-analysis-service` no monta el dominio** (substrate-only en producción).
15. **Materialization service no centralizado** — cada handler self-materializa, sin scheduler de refresh.

---

## ✅ Plan exhaustivo para llegar a 10 / 10

> Las tareas están agrupadas por categoría y priorizadas. No es necesario respetar el orden, pero las primeras categorías son las que más mueven el score. Cada tarea está pensada para ser un PR/épica concreta.

### A. Type system y modelado de objetos

- [ ] **A1.** Añadir `Status` (`ACTIVE | DEPRECATED | EXPERIMENTAL | RETIRED`) y `LifecycleStage` a `models/object_type.go`, `link_type.go`, `property.go`, `interface.go`, `shared_property.go`, `action_type.go`, `function_package.go`, con migrations PG y enforcement en handlers.
- [ ] **A2.** Soportar **composite primary keys**: cambiar `PrimaryKeyProperty string` por `PrimaryKey []string` en `ObjectType`, propagar a `Binding.PrimaryKeyColumns`, ajustar `pg_repository.go` y `cassandra-kernel/object_store.go` (clustering keys).
- [ ] **A3.** Añadir tipos primitivos faltantes al `type_system.go`:
    - `enum` (con `allowed_values []string`)
    - `union` / `oneof` (con `member_types []PropertyType`)
    - `decimal` (con precision/scale)
    - `duration`, `interval`, `date_range`, `time_range`
    - `currency` (con código ISO 4217 y `Money{amount, currency}` struct)
    - `geo_polygon`, `geo_line`, `geo_box`, `h3_cell`
    - `ip_address`, `email`, `phone_e164`, `url`
    - `binary` (con MIME type y tamaño máximo)
- [ ] **A4.** Sistema de **unidades y dimensiones**: librería `libs/units` con conversiones (kg↔lb, m↔ft, USD↔EUR con tipo de cambio); `Property.Unit string` opcional.
- [ ] **A5.** **Tipos paramétricos / genéricos**: soportar `List<T>`, `Map<K,V>`, `Optional<T>` con validación recursiva en `schema.go`.
- [ ] **A6.** **Type hierarchy**: añadir `ObjectType.Extends *string` y `Interface.Extends []string`; resolución transitiva en `domain/schema.go::EffectiveProperties`.
- [ ] **A7.** **Polymorphic links**: cambiar `LinkType.SourceTypeID/TargetTypeID` a `[]string` (union types); validar en `composition.go::CreateLink`.
- [ ] **A8.** **Link properties**: añadir `Properties []PropertyDefinition` a `LinkType` y `payload jsonb` validado en `LinkInstance`.
- [ ] **A9.** **Derived / computed properties**: nuevo módulo `libs/ontology-kernel/expressions` con un mini-DSL (parser + evaluador) y `Property.ComputedExpression *string`. Soportar:
    - Aritmética y string ops sobre otras props
    - `now()`, `today()`, ventanas temporales
    - Agregación cross-link (`count(links_outgoing[type=X])`)
- [ ] **A10.** **Encryptable / Sensitive flags** a nivel propiedad (`Property.Sensitivity`: `Public | Internal | Confidential | Restricted | Secret`) con cifrado en reposo via envelope encryption (KMS).
- [ ] **A11.** **Property-level markings**: marcar propiedades individuales con markings; en `ObjectStore.Get` y `search`, filtrar/redactar campos por marking del caller.
- [ ] **A12.** **Semantic types / tagging**: campo `Property.SemanticTags []string` (`pii.email`, `gdpr.subject`, `hipaa.phi`, `pci.pan`) con DLP hooks.
- [ ] **A13.** **Auto-detección de PII** en ingest: integrar Microsoft Presidio o equivalente Go, marcando propiedades con `Sensitivity` automática y emitiendo evento `ontology.dlp.detected.v1`.
- [ ] **A14.** **Schema versioning**: tabla `ontology_schema_versions` con `(object_type_id, version, schema_jsonb, created_at)`; migrations automáticas con `up`/`down` SQL.
- [ ] **A15.** **Recursive types**: soportar referencias a sí mismo (árboles, grafos jerárquicos) con detección de ciclos.

### B. Validación y enforcement

- [ ] **B1.** Validar **cardinality** en `composition.go::CreateLink`: rechazar si `1:1` y ya existe link, etc.
- [ ] **B2.** Validar que `from`/`to` de un link sean del tipo declarado en el `LinkType`.
- [ ] **B3.** **Check constraints** declarativos en `Property.ValidationRules` con DSL evaluable (no `json.RawMessage` opaco): `min`, `max`, `regex`, `length`, `oneOf`, `dependsOn`.
- [ ] **B4.** **Foreign-key references**: tipo `reference` debe validar existencia y permisos del objeto referenciado.
- [ ] **B5.** **Cascade rules**: `LinkType.OnDelete` (`cascade | restrict | set_null`).
- [ ] **B6.** **Schema migration framework**: cuando una property cambia de tipo, generar plan de migración con `dryRun`/`apply`.

### C. Query language (OQL) y álgebra de Object Sets

- [ ] **C1.** Diseñar e implementar **OQL** (Ontology Query Language) — parser ANTLR/PEG, AST, planner, optimizer, ejecutor sobre `libs/query-engine`. Sintaxis tipo:
    ```
    SELECT name, age FROM Person
    WHERE age > 18 AND department.name = "Eng"
    TRAVERSE manages MAX 3 HOPS
    ORDER BY hire_date DESC
    LIMIT 100
    ```
- [ ] **C2.** **Álgebra de Object Sets** completa en `domain/object_sets.go`:
    - `Union(a, b)` / `Intersect(a, b)` / `Difference(a, b)` / `SymmetricDifference(a, b)`
    - `Map(set, projection)` / `Filter(set, pred)` / `FlatMap`
    - `GroupBy(set, key) -> AggregatedSet`
    - `OrderBy(set, key, dir)` / `Limit(set, n)` / `Sample(set, n)`
- [ ] **C3.** **Funciones de agregación**: `count`, `sum`, `avg`, `min`, `max`, `percentile`, `histogram`, `count_distinct`, `approx_count_distinct (HyperLogLog)`.
- [ ] **C4.** **Window functions**: `LAG`, `LEAD`, `ROW_NUMBER`, `RANK`, `PERCENT_RANK`, `CUMULATIVE_SUM`.
- [ ] **C5.** **Time-series aggregations**: `tumbling`, `sliding`, `hopping` windows; resampling, gap-filling.
- [ ] **C6.** **Join algebra completa**: `inner`, `left`, `right`, `full outer`, `cross`, `semi`, `anti`.
- [ ] **C7.** **Subqueries y CTEs** en OQL.
- [ ] **C8.** **Query planner** con cost-based optimization: estadísticas en `pg_stats`-like sobre Cassandra/PG, join reorder, predicate pushdown a Vespa/OpenSearch.

### D. Time-travel, versionado e historia

- [ ] **D1.** **Bitemporal storage**: añadir `valid_from`, `valid_to`, `transaction_time` a cada `ObjectInstance`; tabla `objects_history` en PG / `objects_by_id_history` en Cassandra.
- [ ] **D2.** **API time-travel**: `GET /objects/{id}?as_of=2025-12-01T00:00:00Z` y `?between=...,...`.
- [ ] **D3.** **OQL time-travel**: `SELECT * FROM Person AS OF 2025-12-01 ...`.
- [ ] **D4.** **Diff API**: `GET /objects/{id}/diff?from=v1&to=v2` con patches RFC 6902.
- [ ] **D5.** **Audit trail por campo**: registrar `changed_by`, `changed_at`, `previous_value`, `new_value`, `reason` en cada update; cero perdida.
- [ ] **D6.** **Retention policies** por ObjectType (e.g., 7 años para finance, 1 año para logs).
- [ ] **D7.** **Soft-delete con tombstones reversibles** y compactación programada.

### E. Markings, ACL y privacidad

- [ ] **E1.** **Marking hierarchy**: `Marking.Parents []MarkingID`; closure transitiva precomputada en `ontology_marking_closure`.
- [ ] **E2.** **Marking categories**: agrupar markings (Compartmented, Need-to-Know, Releasability, Handling Caveat).
- [ ] **E3.** **Auto-propagación de markings**: cuando un objeto se crea por una rule/action a partir de upstream marcados, heredar markings y registrarlo como `MarkingSource::InheritedFromUpstream(ridList)`.
- [ ] **E4.** **Property-level markings enforcement** en `ObjectStore.Get`, `Search`, `OQL` y `Traversal`: redactar campos cuyo marking exceda la clearance del caller.
- [ ] **E5.** **Row-level security predicates**: añadir `RLSPolicy` a ObjectType con expresión Cedar evaluada por fila (`resource.owner == principal.id`).
- [ ] **E6.** **Time-scoped markings**: `Marking.ValidFrom`, `Marking.ValidTo`, `Marking.RevokedAt`.
- [ ] **E7.** **Cedar cableado en handlers**: hoy `authz-cedar-go` no se invoca en cada handler; cablear `IsAuthorized` en `objects/handlers.go`, `actions/`, `links/`, `objectsets/`, `search/`.
- [ ] **E8.** **Differential privacy**: integrar Google DP / OpenDP para queries de agregación; presupuesto epsilon por usuario/query.
- [ ] **E9.** **k-anonymity / l-diversity**: en queries de agregación, requerir `k>=N` para resultados.
- [ ] **E10.** **Field-level encryption (envelope)**: KMS-backed; rotación de DEK; audit de accesos.
- [ ] **E11.** **DLP scanner** integrado al ingest pipeline.
- [ ] **E12.** **Right-to-be-forgotten** (GDPR): API `DELETE /subjects/{id}/forget` que tachoniza por todo el lineage.

### F. Actions, simulación y branching

- [ ] **F1.** **What-if branches reales**: hoy son cosméticas. Implementar copy-on-write sobre ObjectStore (overlay Cassandra), evaluación de rules/actions en branch, merge con resolución de conflictos.
- [ ] **F2.** **Approval workflow**: `Action.RequiresApproval`, `Action.Approvers`, estado `PENDING_APPROVAL → APPROVED → APPLIED`, integración con notification-alerting-service.
- [ ] **F3.** **Async / long-running actions** vía Temporal o `workflow-automation-service`: estado persistente, retries, deadlines, cancel.
- [ ] **F4.** **Action templates**: librería de actions reutilizables con parámetros tipados; `ActionTemplate` y `ActionInstance`.
- [ ] **F5.** **Action composition / sub-actions**: una action puede invocar otras como `sub_action_1, sub_action_2` con manejo transaccional saga.
- [ ] **F6.** **Compensation handlers** explícitos por action.
- [ ] **F7.** **Action scheduling**: `ScheduleAction(action, cron, args)` con `scheduling-cron`.
- [ ] **F8.** **Side-effect declaration**: `Action.SideEffects []SideEffectDescriptor` (writes, deletes, external_calls, notifications) para análisis de impacto pre-commit.
- [ ] **F9.** **Action preview con dry-run completo**: hoy hay simulación parcial; ampliar para mostrar diff exacto, rules disparadas, alertas que se emitirían.

### G. Rules engine

- [ ] **G1.** **Rule composition**: `RuleGroup`, `RuleSet`, importación entre proyectos.
- [ ] **G2.** **Rules con ApplyEffect realmente cableado**: hoy es parcial; añadir worker pool que consume `ontology_rule_schedules` y aplica `object_patch` con writeback transaccional.
- [ ] **G3.** **Rule simulation con branch**: ver qué objetos cambiarían sin aplicar.
- [ ] **G4.** **Bidirectional rules**: triggers no sólo por cambio sino por arrival de eventos externos (Kafka topic, webhook).
- [ ] **G5.** **Rule priority y conflict resolution**: si dos rules patchan el mismo campo, política de precedencia explícita.
- [ ] **G6.** **Async/long-running effects**: efecto `invoke_workflow` que dispara un workflow en `workflow-automation-service`.
- [ ] **G7.** **Rule lineage**: registrar qué rule mutó qué objeto y por qué; visible en object view timeline.

### H. Functions y código pluggable

- [ ] **H1.** **Python sidecar siempre cableado** (no `ErrPythonRuntimeNotWired`); container con isolation (gVisor/Firecracker), límites de CPU/RAM/disk, network egress allowlist.
- [ ] **H2.** **WebAssembly runtime** para functions deterministas (`wasmtime` Go) para casos donde Python/TS sea overkill.
- [ ] **H3.** **Function packages como artefactos versionados** en OCI registry; `FunctionPackage.OCIRef`, deploy via `model-deployment-service`.
- [ ] **H4.** **Dependency management** real: `requirements.txt` / `package.json` resueltos y cacheados; lockfiles.
- [ ] **H5.** **SDK oficiales** generados con `services/sdk-generation-service`:
    - Python (`openfoundry-sdk` con tipos generados desde la ontología)
    - TypeScript (`@openfoundry/sdk` con type-safe Object/Action accessors)
    - Go (`github.com/openfoundry/sdk-go`)
    - Java/Kotlin
- [ ] **H6.** **Function authoring UI**: editor con autocompletion de la ontología, type-checking en vivo, dry-run.
- [ ] **H7.** **Function cost tracking**: CPU·s, memory·s, network bytes, llamadas a LLM por invocación.
- [ ] **H8.** **Function tracing**: spans OTLP por invocación, logs persistidos por run.
- [ ] **H9.** **Function caching**: si la entrada es la misma y la función es marcada `pure`, devolver resultado cacheado.

### I. Search

- [ ] **I1.** **Phonograph / phonetic search**: integrar Soundex, Metaphone, Double Metaphone, BeiderMorse en Vespa (custom tokenizer) y OpenSearch (analyzer).
- [ ] **I2.** **Fuzzy / wildcard / proximity / phrase search** expuestos en la abstraction (`SearchQuery.Fuzzy`, `Wildcard`, `Proximity`).
- [ ] **I3.** **Geo search**: `geo_distance`, `geo_bbox`, `geo_polygon`, `geo_shape` predicates.
- [ ] **I4.** **Faceted navigation completa**: aggregations multi-valor, ranges, date histograms, nested facets.
- [ ] **I5.** **Typed search** con narrowing en query compilation (e.g., `kind:Person AND department:Eng`).
- [ ] **I6.** **Autocomplete / search-as-you-type** con n-grams o completion suggester.
- [ ] **I7.** **Custom analyzers** por property: stemming, synonyms, stop-words, multi-language.
- [ ] **I8.** **Search explain** expuesto en API para debugging de scoring.
- [ ] **I9.** **Learning-to-rank**: cross-encoder reranking (BGE-reranker, Cohere); pipeline de training con clicks/feedback.
- [ ] **I10.** **Highlighting** real (no sólo snippets), con tokens marcados por relevance.
- [ ] **I11.** **Search streaming** (cursor-based) para resultados grandes.
- [ ] **I12.** **Saved searches** y subscriptions a cambios en resultados.

### J. Vector / semantic / AI

- [ ] **J1.** **Embedding generation pipeline**: integraciones a OpenAI, Cohere, VoyageAI, modelos locales (BGE, E5); workers que rellenan embeddings al ingest.
- [ ] **J2.** **Embedding versioning**: `Embedding.ModelVersion`; reindex programado al cambiar modelo.
- [ ] **J3.** **pgvector backend** real (hoy es skeleton): `services/object-database-service` con extensión `pgvector`, índices `HNSW`/`IVFFlat`.
- [ ] **J4.** **Hybrid retrieval con RRF / RAG**: combinación BM25 + dense + sparse (SPLADE) con Reciprocal Rank Fusion configurable.
- [ ] **J5.** **Reranking** bi-encoder y cross-encoder.
- [ ] **J6.** **Semantic caching** de queries frecuentes con similitud vectorial.
- [ ] **J7.** **RAG evaluation harness**: BLEU, ROUGE, faithfulness, context relevance; integrado a `ai-evaluation-service`.
- [ ] **J8.** **AIP-style**: chat-with-ontology endpoint que combina OQL + RAG + tool-use sobre actions.

### K. Geospatial

- [ ] **K1.** **`libs/geospatial-core` real**: integrar `paulmach/orb`, `golang/geo`, `uber/h3-go`.
- [ ] **K2.** **PostGIS** en `services/object-database-service` (tabla con `GEOMETRY(...)`).
- [ ] **K3.** **Predicates geo en OQL y Object Sets**: `ST_Within`, `ST_Contains`, `ST_Distance`, `ST_DWithin`.
- [ ] **K4.** **H3 / S2 indexing** para queries de proximidad y heatmaps.
- [ ] **K5.** **Geotime (Gotham-style)**: trayectorias de objetos a lo largo del tiempo, con queries `objects_within(polygon, time_range)`.
- [ ] **K6.** **MVT tile generation** en `geospatial-tiles` para mapas zoom-able.
- [ ] **K7.** **Reverse geocoding** y autocomplete de direcciones.
- [ ] **K8.** **Map layers** y heatmaps en `ontology-exploratory-analysis-service` cableados al router.

### L. Data binding, ETL e ingest

- [ ] **L1.** **Transform DSL** en `Binding.Transform`: expresiones (no sólo column→property mapping) con funciones `lower`, `upper`, `coalesce`, `parse_date`, `to_geo_point`, etc.
- [ ] **L2.** **CDC nativo** desde Cassandra (CDC table) y otros stores; conectores en `ingestion-replication-service`.
- [ ] **L3.** **Magritte-equivalent**: pipeline visual para definir bindings con preview en vivo del dataset.
- [ ] **L4.** **Bulk import API**: `POST /tenants/{t}/objects/_bulk` con NDJSON streaming, dedup, error reporting por línea.
- [ ] **L5.** **Streaming ingest**: Kafka topic → ObjectStore con transformación por evento.
- [ ] **L6.** **Schema inference** automática desde dataset.
- [ ] **L7.** **Data quality checks** integradas: not-null, unique, range, regex, referential integrity; resultado expuesto como `Funnel.HealthCheck`.
- [ ] **L8.** **Backfill / replay** programable cuando se cambia un binding.

### M. Materializations y performance

- [ ] **M1.** **Materializations service centralizado**: `services/materializations-service` que ejecuta refresh schedules, compactación, tracking de cost.
- [ ] **M2.** **Incremental materializations** (delta) para Object Sets grandes.
- [ ] **M3.** **Cache layer** (Redis) para object reads y queries calientes.
- [ ] **M4.** **Query result caching** con invalidación basada en lineage de la query.
- [ ] **M5.** **Read replicas** Cassandra y PG para hot reads.
- [ ] **M6.** **Materialization cost metrics** y alerts.
- [ ] **M7.** **TTL automático** para materializations no consultadas.

### N. Branching, versionado y release management

- [ ] **N1.** **Ontology versioning** semántico (`v1.4.2`) con `release notes` autogenerados.
- [ ] **N2.** **Stable releases** con freeze de schema; rollback completo.
- [ ] **N3.** **Approval chains** en `OntologyProjectProposal` (multi-reviewer, secuencial o paralelo, requirements de roles).
- [ ] **N4.** **Diff visualizer** entre branches/versions: qué tipos cambiaron, qué propiedades, qué links.
- [ ] **N5.** **Migration plans** auto-generados al hacer merge de un branch (cómo migrar datos existentes).
- [ ] **N6.** **Configuration as Code**: DSL YAML/HCL para definir la ontología, con `terraform-style apply/plan`.
- [ ] **N7.** **GitOps**: sincronización con un repo Git canónico de la ontología.
- [ ] **N8.** **Promotion model**: `dev → staging → prod` con quality gates.

### O. Lineage end-to-end

- [ ] **O1.** **Lineage de objetos**: además de dataset/columna, lineage de cambios por objeto (qué action lo modificó, qué rule, qué pipeline).
- [ ] **O2.** **Lineage de markings** end-to-end: si redefino un upstream marking, qué objetos derivados lo heredan.
- [ ] **O3.** **Impact analysis bidireccional**: dado un objeto, qué dashboards/reports/functions se rompen si lo borro.
- [ ] **O4.** **OpenLineage compliance** completo (ya hay headers; expandir a todas las operaciones).
- [ ] **O5.** **Lineage UI** integrado en object views.

### P. Observability y operations

- [ ] **P1.** **W3C Trace Context propagation** entre servicios (hoy OTLP es básico).
- [ ] **P2.** **Multi-tenant observability isolation**: tenant labels propagados en traces/metrics.
- [ ] **P3.** **SLO / Error budget** por servicio con alerts.
- [ ] **P4.** **Saga real-time monitoring dashboard** con state de cada saga en vuelo.
- [ ] **P5.** **DLQ automático** en consumers Kafka.
- [ ] **P6.** **Backpressure** y rate-limiting dinámico.
- [ ] **P7.** **Exactly-once semantics** end-to-end (Kafka transactions + idempotency).
- [ ] **P8.** **Saga compensation timeouts** configurables.
- [ ] **P9.** **Circuit breakers** por dependencia.
- [ ] **P10.** **Chaos engineering harness** (failure injection en CI).
- [ ] **P11.** **Cost attribution** por tenant/project/action/function.
- [ ] **P12.** **Audit anomaly detection** (ML sobre `of_audit.events`).

### Q. APIs, SDKs y experiencia del cliente

- [ ] **Q1.** **gRPC + REST + GraphQL** todos vivos para la ontología.
- [ ] **Q2.** **Subscriptions / streaming** vía gRPC server-streaming o WebSockets para cambios de objetos en vivo.
- [ ] **Q3.** **OpenAPI 3.1 schema** completo y publicado.
- [ ] **Q4.** **SDKs oficiales** Python/TypeScript/Go/Java con type-generation desde ontología.
- [ ] **Q5.** **CLI oficial** `of` para administrar la ontología.
- [ ] **Q6.** **Webhook subscriptions** para eventos del control-plane (object created/updated, action applied, rule fired).

### R. Workflow / Funnel

- [ ] **R1.** **Visual workflow editor** integrado a Funnel.
- [ ] **R2.** **Approval gates** en workflow steps.
- [ ] **R3.** **Conditional branches** y loops.
- [ ] **R4.** **Funnel SLA tracking** con alertas de breach.
- [ ] **R5.** **Funnel templates / catalog** reutilizables.

### S. Front-end y UX

- [ ] **S1.** **Object Explorer** UI con search, filters, facets, graph view.
- [ ] **S2.** **Schema designer** UI (drag-and-drop ObjectTypes/Properties/Links).
- [ ] **S3.** **Action runner** UI con form generation desde `FormSchema`.
- [ ] **S4.** **Rules workbench** UI con simulator.
- [ ] **S5.** **Quiver UI** con time-series interactivos (zoom, brush, multi-axis).
- [ ] **S6.** **Map / Geotime UI** con timeline scrubber.
- [ ] **S7.** **Object views** UI configurables por tipo.

### T. Testing, calidad y documentación

- [ ] **T1.** **Contract tests** wire-format Rust↔Go bit a bit (existen algunos; expandir a 100% de los handlers).
- [ ] **T2.** **End-to-end tests** con `compose.yaml` real (Cassandra + Vespa + PG + Kafka + Cedar).
- [ ] **T3.** **Property-based testing** (Rapid) para validators y query engine.
- [ ] **T4.** **Performance benchmarks** continuos con regression alerts (ya hay `benchmarks/`; expandir).
- [ ] **T5.** **Fuzzing** de OQL parser y JSON validators.
- [ ] **T6.** **Security tests**: PII leak detection, marking bypass attempts, Cedar policy edge cases.
- [ ] **T7.** **Documentación de referencia** completa por módulo.
- [ ] **T8.** **Tutoriales** end-to-end (definir un ObjectType → bind dataset → action → rule → object set → query).
- [ ] **T9.** **Conformance suite** publicable para validar implementaciones.

### U. Multi-region, escala y disponibilidad

- [ ] **U1.** **Multi-region replication** Cassandra (NetworkTopologyStrategy) y Vespa (cross-region content clusters).
- [ ] **U2.** **Active-active** con conflict resolution determinista.
- [ ] **U3.** **Disaster recovery** con RPO/RTO targets explícitos.
- [ ] **U4.** **Tenant sharding** automatizado para escalar horizontalmente.
- [ ] **U5.** **Autoscaling** basado en queue depth y latency.

### V. Compliance y enterprise

- [ ] **V1.** **SOC2 / ISO27001** evidence collection automatizada.
- [ ] **V2.** **HIPAA / GDPR / CCPA** templates de policies y data residency controls.
- [ ] **V3.** **FIPS 140-2** crypto compliance.
- [ ] **V4.** **STIG** hardening de containers.
- [ ] **V5.** **Air-gapped deployment** support (no llamadas a internet).
- [ ] **V6.** **Bring-your-own KMS** para customer-managed keys.

---

## 🧭 Roadmap sugerido (orden por impacto en score)

**Sprint 1–2 (mover a 7/10):** A1, A2, A3, A6, A7, A8, A9, B1, B2, C2, F1, G2, H1, E7.
**Sprint 3–4 (mover a 8/10):** C1 (OQL básico), D1–D4 (time-travel), E1–E5 (markings avanzados), I1–I4, K1–K4.
**Sprint 5–6 (mover a 9/10):** C3–C8, D5–D7, E8–E12, F2–F9, J1–J6, M1–M3, N1–N6.
**Sprint 7+ (cerrar a 10/10):** todo lo restante (multi-region, compliance, UX completo, conformance suite).

---

## 📌 Notas finales

- La arquitectura subyacente (Cassandra + PG + Vespa + Cedar + Outbox + Saga + Kafka + Iceberg) **es la correcta** y está al nivel de Foundry. Lo que falta es **profundidad en las capacidades** y **cierre de los stubs** (what-if, Python runtime, exploratory-analysis router, Cedar en handlers, ApplyRuleEffect end-to-end).
- Hay muchos archivos `*_test.go` en `models/` y `domain/`, lo cual es excelente; mantener esa cultura al añadir las features de arriba.
- El branching/proposals está más avanzado que en otros clones: aprovecharlo para construir encima `what-if branches reales`, `approval chains` y `migration plans` (gana mucho con poca duplicación).
- Recomendación operativa: NO atacar las 200+ tareas en orden lineal; agrupar por épicas (`OQL`, `Time-travel`, `Markings 2.0`, `Geospatial`, `Functions runtime`) y entregar cada épica completa antes de pasar a la siguiente, para evitar half-finished features.

---

**Veredicto final:** OpenFoundry está en el percentil ~75 de los clones open-source de Palantir Foundry que existen, pero el camino de 6 → 10 es largo y exige las ~200 tareas de arriba. El esqueleto y la arquitectura son sólidos; el trabajo restante es **rellenar capacidades** y **cerrar stubs**, no rediseñar.
