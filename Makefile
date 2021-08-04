SHELL := powershell

ifeq ($(OS),Windows_NT)     # is Windows_NT on XP, 2000, 7, Vista, 10...
	export CGO_CPPFLAGS = -IC:/libjpeg-turbo-gcc64/include
	export CGO_LDFLAGS = -Wl,-Bstatic -LC:/libjpeg-turbo-gcc64/lib
endif

all: build

dev:
	CompileDaemon -exclude=ui/* -exclude=.git/* -include=*.yaml -command=./photofield -log-prefix=false
.PHONY: dev

export_grafana:
	@hamara export --host=localhost:9091 --key=${GRAFANA_API_KEY} > docker/grafana/provisioning/datasources/default.yaml
.PHONY: export_grafana

db_new:
	migrate create -ext sql -dir db/migrations -seq $(name)
.PHONY: db_new

db_up:
	migrate -database sqlite://data/photofield.cache.db -path db/migrations up
.PHONY: db_new

db_down:
	migrate -database sqlite://data/photofield.cache.db -path db/migrations down
.PHONY: db_new

ui:
	cd ui && \
	npm run dev
.PHONY: ui

build:
# go build -tags libjpeg
	go build -buildmode=exe
.PHONY: build

run: build
	./photofield
.PHONY: run
