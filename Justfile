build:
	go build -o choo ./cmd/choo

test:
	go test ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

ci: fmt vet test

install:
	go install ./cmd/choo

clean:
	rm -f choo

build-release:
	@version=$$(git describe --tags --always --dirty); \
	commit=$$(git rev-parse HEAD); \
	date=$$(date -u +%Y-%m-%dT%H:%M:%SZ); \
	go build -ldflags="-X 'main.version=$$version' -X 'main.commit=$$commit' -X 'main.date=$$date'" -o choo ./cmd/choo
