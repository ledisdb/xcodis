all: build

build: build-proxy build-config 

build-proxy:
	go build -o bin/codis-proxy ./cmd/proxy

build-config:
	go build -o bin/codis-config ./cmd/cconfig

clean:
	@rm -rf bin
	@rm -f *.rdb *.out *.log *.dump 
	@if [ -d test ]; then cd test && rm -f *.out *.log *.rdb; fi

test:
	go test ./... -race
