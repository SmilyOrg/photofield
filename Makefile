SHELL := powershell

ifeq ($(OS),Windows_NT)     # is Windows_NT on XP, 2000, 7, Vista, 10...
	export CGO_CPPFLAGS = -IC:/libjpeg-turbo-gcc64/include
	export CGO_LDFLAGS = -Wl,-Bstatic -LC:/libjpeg-turbo-gcc64/lib
endif

all: build

dev:
	echo $(detected_OS)
	CompileDaemon -exclude=ui/* -exclude=.git/* -include=*.yaml -command=./photofield -log-prefix=false
.PHONY: dev

build:
	go build -tags libjpeg
.PHONY: build

run: build
	./photofield
.PHONY: run
