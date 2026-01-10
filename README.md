# Doclet

Simple, anonymous, real-time rich-text collaboration.

## Local setup

### 1) Start dependencies
```sh
docker compose up -d
```

### 2) Set environment variables
```sh
cp .env.example .env
```
Edit `.env` as needed. Defaults assume local Postgres + NATS.

### 3) Run the Go services
```sh
export $(grep -v '^#' .env | xargs)

go run ./cmd/document
```
In another terminal:
```sh
export $(grep -v '^#' .env | xargs)

go run ./cmd/collab
```

### 4) Run the frontend
```sh
cd frontend
npm install
npm run dev
```

Open `http://localhost:5173`.

## Useful commands
- `go test ./...`
- `docker compose ps`
- `docker compose down -v`

## Notes
- Document schema is code-first (Gorm). Migrations are generated via `go run ./services/document/cmd/atlas`.
- Snapshots are saved via NATS on a short debounce from the editor.
