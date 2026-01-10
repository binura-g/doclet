# Architecture Overview

## Summary
Doclet uses a simple two-service backend: a document service for persistence and a collaboration service for WebSocket sessions. NATS acts as a message bus for real-time fan-out across replicas. PostgreSQL stores document metadata and content. The frontend is a React app with TipTap + Yjs.

## Components
### Frontend (React)
- Two pages: Home (list + search) and Editor.
- Connects to Document Service via REST for list/load/create.
- Connects to Collaboration Service via WebSocket for live updates.
- Applies inbound updates to the editor state.
- Renders presence (cursor and activity) for each anonymous session.
- Uses Yjs awareness for cursors and user activity.

### Document Service (Go)
- REST API for list/create/load document with an OpenAPI spec.
- Persists `document_id`, `displayName`, and `content` in PostgreSQL.
- Subscribes to NATS update events to persist changes.
- Uses Gorm models; migrations are generated from code via Atlas.

### Collaboration Service (Go)
- Manages WebSocket connections per `document_id`.
- Receives Yjs updates from clients and publishes to NATS.
- Subscribes to NATS to forward updates to local clients.
- Manages presence: cursor position, selection, and activity pings.

### PostgreSQL
- Primary data store for documents.
- Table: `documents(document_id UUID PK, display_name TEXT, content BYTEA, updated_at TIMESTAMPTZ)`.

### Database Migrations (Code-First)
- Schema changes are expressed in code and generate migration files.
- Migrations are applied by the Document Service during development and CI.
- Tooling choice: Atlas.
- Gorm `AutoMigrate` runs on startup for local development.

### NATS
- Pub/Sub for document updates.
- Topic pattern: `doclet.documents.<document_id>.updates`, `.presence`, `.snapshots`.
- Enables horizontal scaling and resilience.

## Data Flow
### Create Document
1. Client calls `POST /documents` with optional `displayName`.
2. Document Service generates `document_id`, stores initial content, returns payload.

### Join Document
1. Client calls `GET /documents/{document_id}`.
2. Client opens WebSocket to Collaboration Service with `document_id`.

### Edit Propagation
1. Client emits edit operations over WebSocket.
2. Collaboration Service publishes update to NATS.
3. All Collaboration Service replicas receive update and forward to connected clients.
4. Client periodically sends Yjs snapshots for persistence.
5. Document Service consumes snapshot events and persists content.

## API Sketch
### REST
- `POST /documents` -> `{ document_id, displayName, content }`
- `GET /documents/{document_id}` -> `{ document_id, displayName, content }`
- `GET /documents?query=plan` -> list/search by `displayName`, sorted by `updated_at` desc

### WebSocket
- Connect: `/ws?document_id=<id>`
- Messages:
  - `yjs_update`: `{ document_id, payload, client_id }` (payload is base64 Yjs update)
  - `presence`: `{ document_id, client_id, payload }` (payload is base64 awareness update)
  - `yjs_snapshot`: `{ document_id, payload, client_id }` (payload is base64 Yjs snapshot)

## Consistency Model
- Collaboration layer relays Yjs updates per session.
- Persistence is eventually consistent (NATS -> Document Service).
- On join, client loads latest persisted content then receives live ops.

## Deployment Notes
- Run multiple collaboration instances behind a load balancer.
- NATS and PostgreSQL are shared services.
- Kubernetes-ready: stateless services, externalized storage.

## Local Development
- `docker compose up -d` starts PostgreSQL and NATS for local work.

## Reliability Considerations
- If a collaboration instance fails, clients reconnect via load balancer.
- NATS ensures updates propagate across instances.
- Document Service can replay from the latest persisted state on recovery.

## Security Considerations
- No auth in MVP; use unguessable UUIDs as access tokens.
- Optional rate limits on create/join to reduce abuse.

## Observability
- Metrics: edit latency, WebSocket connections, NATS publish/consume rates.
- Logs: document_id-scoped edit events and persistence outcomes.

## Open Decisions
- Rich-text engine: Quill.js vs TipTap.
- Operational transform vs CRDT choice and library.
- Persistence frequency and batching strategy.
