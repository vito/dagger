package core

type Socket struct {
	IDable

	HostPath string `json:"host_path,omitempty"`
}

func NewHostSocket(absPath string) *Socket {
	return &Socket{
		HostPath: absPath,
	}
}
