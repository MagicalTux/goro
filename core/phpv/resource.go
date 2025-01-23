package phpv

type ResourceType int

const (
	ResourceUnknown = iota
	ResourceStream
)

func (rs ResourceType) String() string {
	switch rs {
	case ResourceStream:
		return "stream"
	}
	return "unknown"
}

type Resource interface {
	Val
	GetResourceType() ResourceType
	GetResourceID() int
}
