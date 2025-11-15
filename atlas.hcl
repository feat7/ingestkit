// Atlas configuration for IngestKit
// Automatically generates migration SQL by diffing schemas

env "local" {
  // Source: desired schema (from generated/sql/schema.sql)
  src = "file://generated/sql/schema.sql"

  // Target: current database state
  url = "postgres://ingestkit:ingestkit_dev@localhost:5433/ingestkit?sslmode=disable"

  // Migration directory (compatible with golang-migrate)
  dev = "docker://postgres/18/dev"

  migration {
    dir = "file://migrations"
    format = golang-migrate
  }
}

env "docker" {
  // For Docker deployments
  src = "file://generated/sql/schema.sql"
  url = "postgres://ingestkit:ingestkit_dev@postgres:5432/ingestkit?sslmode=disable"
  dev = "docker://postgres/18/dev"

  migration {
    dir = "file://migrations"
    format = golang-migrate
  }
}
