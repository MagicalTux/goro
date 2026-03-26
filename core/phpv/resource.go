package phpv

type ResourceType int

const (
	ResourceUnknown = iota
	ResourceStream
	ResourceContext
	ResourceStreamFilter
)

func (rs ResourceType) String() string {
	switch rs {
	case ResourceStream:
		return "stream"
	case ResourceContext:
		return "stream-context"
	case ResourceStreamFilter:
		return "stream filter"
	}
	return "Unknown"
}

type Resource interface {
	Val
	GetResourceType() ResourceType
	GetResourceID() int
}
