fmt:
	go mod tidy
	gofmt -w .

run:
	go run cmd/netpoltool/main.go --namespace=ns-npt-0 --pod=serve-pod-info --to-namespace=ns-npt-1 --to-pod=serve-pod-info

clitools-build:
	docker build testdata/images/clitools --tag clitools:latest

clitools-push:
	docker tag clitools:latest cheriot/clitools:latest
	docker push cheriot/clitools:latest

convey:
	$$(go env GOPATH)/bin/goconvey
