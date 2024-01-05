package atom

// ReadWriteEvent represents the information associated with a
// read-write event within an AtomGroup; It includes details such as
// the group name, Atom name, previous value, and current value
// involved in the event.
type ReadWriteEvent[T any] struct {
	GroupName string
	AtomName  string
	Previous  *T
	Current   *T
}

// AtomGroup represents a collection of Atom instances that are
// associated and can be used to perform group-level operations; It
// allows the creation of named Atom instances within the group, and
// provides a mechanism to set a callback function to be invoked on
// every read-write operation within the group.
type AtomGroup[T any] struct {
	name        string
	onReadWrite func(ReadWriteEvent[T])
}

func NewAtomGroup[T any](name string) AtomGroup[T] {
	return AtomGroup[T]{
		name: name,
	}
}

func (this *AtomGroup[T]) New(name string, value T) Atom[T] {
	atom := New(value)
	atom.name = &name
	atom.group = this
	return atom
}

func (this *AtomGroup[T]) Dead() Atom[T] {
	return Dead[T]()
}

// OnReadWrite sets a callback function to be invoked on every
// read-write operation within the AtomGroup.
func (this *AtomGroup[T]) OnReadWrite(callback func(ReadWriteEvent[T])) {
	this.onReadWrite = callback
}

// DoReadWrite invokes the OnReadWrite callback function, if set, with
// the information about a read-write event within the AtomGroup; It
// provides details such as the group name, Atom name, previous value,
// and current value; If no callback is set, this method has no
// effect.
func (this *AtomGroup[T]) DoReadWrite(name string, previous *T, current *T) {
	if this.onReadWrite != nil {
		event := ReadWriteEvent[T]{
			GroupName: this.name,
			AtomName:  name,
			Previous:  previous,
			Current:   current,
		}
		this.onReadWrite(event)
	}
}
