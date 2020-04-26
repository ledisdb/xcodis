export GO111MODULE=on

PACKAGES ?= $(shell GO111MODULE=on go list -mod=vendor ./... | grep -v /vendor/)

all: build

build: build-proxy build-config build-ha

build-proxy:
	GO111MODULE=on go build -mod=vendor -o bin/codis-proxy ./cmd/proxy

build-config:
	GO111MODULE=on go build -mod=vendor -o bin/codis-config ./cmd/cconfig

build-ha:
	GO111MODULE=on go build -mod=vendor -o bin/codis-ha ./cmd/ha

update:
	go mod tidy -v && go mod vendor

clean:
	GO111MODULE=on go clean -i ./...
	@rm -rf bin
	@rm -f *.rdb *.out *.log *.dump 
	@if [ -d test ]; then cd test && rm -f *.out *.log *.rdb; fi

fmt:
	gofmt -w -s  . 2>&1 | grep -vE 'vendor' | awk '{print} END{if(NR>0) {exit 1}}'

vet:
	GO111MODULE=on go vet -mod=vendor $(PACKAGES)

test:
	GO111MODULE=on go test -mod=vendor -race -cover -coverprofile coverage.out $(PACKAGES)
