build:
	export GOSUMDB=off
	go mod vendor
	go mod tidy
	go vet
	go build -mod=vendor -o=exe