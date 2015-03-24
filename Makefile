all: build

build: build-proxy build-config build-ha

build-proxy:
	godep go build -o bin/codis-proxy ./cmd/proxy

build-config:
	godep go build -o bin/codis-config ./cmd/cconfig

build-ha:
	godep go build -o bin/codis-ha ./cmd/ha

clean:
	godep go clean -i ./...
	@rm -rf bin
	@rm -f *.rdb *.out *.log *.dump 
	@if [ -d test ]; then cd test && rm -f *.out *.log *.rdb; fi

test:
	godep go test ./... -race 
	godep go test ./proxy/router -broker="ledisdb"
