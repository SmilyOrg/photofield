set dotenv-load := true

default:
  @just --list --list-heading $'photofield\n'

build *args:
  go build {{args}}

build-ui:
  cd ui && npm run build

build-local:
  goreleaser build --snapshot --single-target --rm-dist

release-local:
  goreleaser release --snapshot --skip-publish --rm-dist

run *args: build
  ./photofield {{args}}

run-static *args:
  go build -tags embedstatic
  ./photofield {{args}}

ui:
  cd ui && npm run dev

watch:
  watchexec --exts go,yaml -r just run

db-add migration_file_name:
  migrate create -ext sql -dir db/migrations -seq {{migration_file_name}}

db *args:
  migrate -database sqlite://data/photofield.cache.db -path db/migrations {{args}}

api-codegen:
  oapi-codegen -generate="types,chi-server" -package=openapi api.yaml > internal/openapi/api.gen.go

grafana-export:
  @hamara export --host=localhost:9091 --key=$GRAFANA_API_KEY > docker/grafana/provisioning/datasources/default.yaml

pprof := "http://localhost:8080/debug/pprof"

prof-cpu seconds="10":
  mkdir -p profiles/cpu/
  filepath=profiles/cpu/cpu-$(date +"%F-%H%M%S").pprof && \
  curl --progress-bar -o $filepath {{pprof}}/profile?seconds={{seconds}} && \
  go tool pprof -http=: $filepath

prof-heap:
  mkdir -p profiles/heap/
  filepath=profiles/heap/heap-$(date +"%F-%H%M%S").pprof && \
  curl --progress-bar -o $filepath {{pprof}}/heap && \
  go tool pprof -http=: $filepath
