package netlink

//go:generate go run github.com/golang/mock/mockgen -destination mocks/netlink_mocks.go -copyright_file ../../../hack/boilerplate.go.txt . NetLink
//go:generate go run github.com/golang/mock/mockgen -destination mocks/link_mocks.go -copyright_file ../../../hack/boilerplate.go.txt github.com/vishvananda/netlink Link
