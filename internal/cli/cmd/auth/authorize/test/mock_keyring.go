package test

type MockKeyring struct {
	key   string
	value []byte
}

func (m *MockKeyring) Set(value []byte) error {
	m.value = value
	return nil
}

func (m *MockKeyring) Get() ([]byte, error) {
	return m.value, nil
}

func (m *MockKeyring) Remove() error {
	m.key = ""
	m.value = nil
	return nil
}

func (m *MockKeyring) IsValid() bool {
	return true
}
