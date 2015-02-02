all: build

build: build-proxy build-config 

build-proxy:
	godep go build -o bin/codis-proxy ./cmd/proxy

build-config:
	godep go build -o bin/codis-config ./cmd/cconfig

clean:
	godep go clean -i ./...
	@rm -rf bin
	@rm -f *.rdb *.out *.log *.dump 
	@if [ -d test ]; then cd test && rm -f *.out *.log *.rdb; fi

test:
	godep go test ./... -race
