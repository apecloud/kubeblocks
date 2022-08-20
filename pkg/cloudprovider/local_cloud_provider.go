package cloudprovider

const (
	Local = "local"
)

type localCloudProvider struct {
}

func (p *localCloudProvider) Name() string {
	return Local
}

func (p *localCloudProvider) Apply(destroy bool) error {
	return nil
}

func (p *localCloudProvider) Instance() (Instance, error) {
	return &localInstance{}, nil
}

type localInstance struct {
}

func (l *localInstance) GetIP() string {
	return "127.0.0.1"
}
