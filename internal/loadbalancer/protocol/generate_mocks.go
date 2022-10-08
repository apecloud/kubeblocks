package protocol

//go:generate go run github.com/golang/mock/mockgen -destination mocks/node_client_mocks.go -copyright_file ../../../hack/boilerplate.go.txt . NodeClient
//go:generate go run github.com/golang/mock/mockgen -destination mocks/node_cache_mocks.go -copyright_file ../../../hack/boilerplate.go.txt . NodeCache
