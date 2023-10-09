set dotenv-load := true

default:
  @just --list --list-heading $'photofield\n'

build *args:
  go build {{args}}

build-ui:
  cd ui && npm run build

build-local:
  goreleaser build --snapshot --single-target --clean

# Download geopackage to be embedded via -tags embedgeo
assets:
  mkdir -p data/geo
  gpkg_file="$(grep -e '//go:embed data/geo/' embed-geo.go | cut -d / -f 5)" && \
    gpkg_ver="$(grep -e '// tinygpkg-data release:' embed-geo.go | cut -d ' ' -f 4)" && \
    echo "Downloading $gpkg_ver/$gpkg_file" && \
    wget -O data/geo/$gpkg_file https://github.com/SmilyOrg/tinygpkg-data/releases/download/$gpkg_ver/$gpkg_file

release-local:
  goreleaser release --snapshot --clean

run *args: build
  ./photofield {{args}}

run-static *args:
  go build -tags embedstatic
  ./photofield {{args}}

run-geo *args:
  go build -tags embedgeo
  ./photofield {{args}}

bench collection: build
  ./photofield -bench -bench.collection {{collection}} -test.benchtime 1s -test.count 6

ui:
  cd ui && npm run dev

watch:
  watchexec --exts go,yaml -r just run

db-add migration_file_name:
  migrate create -ext sql -dir db/migrations -seq {{migration_file_name}}

db *args:
  migrate -database sqlite://data/photofield.cache.db -path db/migrations {{args}}

dbt-add migration_file_name:
  migrate create -ext sql -dir db/migrations-thumbs -seq {{migration_file_name}}

dbt *args:
  migrate -database sqlite://data/photofield.thumbs.db -path db/migrations-thumbs {{args}}

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
