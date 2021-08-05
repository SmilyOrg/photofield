set dotenv-load := true

default:
  @just --list --list-heading $'photofield\n'

build:
  go build

run: build
  ./photofield

ui:
  cd ui && npm run dev

watch:
  watchexec --exts go,yaml -r just run

db-add migration_file_name:
  migrate create -ext sql -dir db/migrations -seq {{migration_file_name}}

db *args:
  migrate -database sqlite://data/photofield.cache.db -path db/migrations {{args}}

grafana-export:
  @hamara export --host=localhost:9091 --key=$GRAFANA_API_KEY > docker/grafana/provisioning/datasources/default.yaml
