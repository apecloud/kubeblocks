package agent

//go:generate go run github.com/golang/mock/mockgen -destination mocks/node_manager_mocks.go -copyright_file ../../../hack/boilerplate.go.txt . NodeManager
//go:generate go run github.com/golang/mock/mockgen -destination mocks/node_mocks.go -copyright_file ../../../hack/boilerplate.go.txt . Node
