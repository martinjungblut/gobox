package atom

// Portal is a communication bridge that facilitates interaction
// between two distinct parts of the code. It provides a Reader
// channel for receiving values and a Writer channel for sending
// values. This allows seamless communication and data exchange
// between different components or goroutines.
type Portal[T any] struct {
	Reader <-chan *T
	Writer chan<- *T
}
