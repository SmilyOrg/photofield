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

api-codegen:
  oapi-codegen -generate="types,chi-server" -package=openapi api.yaml > internal/api/api.gen.go

grafana-export:
  @hamara export --host=localhost:9091 --key=$GRAFANA_API_KEY > docker/grafana/provisioning/datasources/default.yaml

pprof := "http://localhost:8080/debug/pprof"

prof-cpu seconds="10":
  filepath=profiles/cpu/cpu-$(date +"%F-%H%M%S").pprof && \
  curl --progress-bar -o $filepath {{pprof}}/profile?seconds={{seconds}} && \
  go tool pprof -http=: $filepath