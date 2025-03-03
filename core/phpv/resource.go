package phpv

type ResourceType int

const (
	ResourceUnknown = iota
	ResourceStream
	ResourceContext
)

func (rs ResourceType) String() string {
	switch rs {
	case ResourceStream:
		return "stream"
	case ResourceContext:
		return "stream-context"
	}
	return "unknown"
}

type Resource interface {
	Val
	GetResourceType() ResourceType
	GetResourceID() int
}
