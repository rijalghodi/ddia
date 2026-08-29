.PHONY: run-appendonly run-segmented test clean

# Run the CLI with the append-only log engine
run-appendonly:
	go run ./cmd/db/main.go --engine=appendonly

# Run the CLI with the segmented log engine
run-segmented:
	go run ./cmd/db/main.go --engine=segmented

# Run all unit tests
test:
	go test -v ./...

# Clean up all generated database data
clean:
	rm -rf ./data
