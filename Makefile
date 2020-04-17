build:
	export GOSUMDB=off
	go mod vendor
	go mod tidy
	go vet
	go env GOOS=darwin build -mod=vendor -o=exe