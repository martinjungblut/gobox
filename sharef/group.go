package sharef

// ReadWriteEvent represents the information associated with a
// read-write event within a Group;
// It includes details such as the group name, Sharef name, previous
// value, and current value involved in the event.
type ReadWriteEvent[T any] struct {
	GroupName  string
	SharefName string
	Previous   *T
	Current    *T
}

// Group represents a collection of Sharef instances that are
// associated and can be used to perform group-level operations;
// It allows the creation of named Sharef instances within the group,
// and provides a mechanism to set a callback function to be invoked
// on every read-write operation within the group.
type Group[T any] struct {
	name        string
	onReadWrite func(ReadWriteEvent[T])
}

func NewGroup[T any](name string) Group[T] {
	return Group[T]{
		name: name,
	}
}

func (this *Group[T]) New(name string, value T) Sharef[T] {
	sharedref := New(value)
	sharedref.name = &name
	sharedref.group = this
	return sharedref
}

// OnReadWrite sets a callback function to be invoked on every
// read-write operation within the Group.
func (this *Group[T]) OnReadWrite(callback func(ReadWriteEvent[T])) {
	this.onReadWrite = callback
}

// doReadWrite invokes the OnReadWrite callback function, if set, with
// the information about a read-write event within the Group;
// It provides details such as the group name, Sharef name, previous
// value, and current value;
// If no callback is set, this method has no effect.
func (this *Group[T]) doReadWrite(name string, previous *T, current *T) {
	if this.onReadWrite != nil {
		event := ReadWriteEvent[T]{
			GroupName:  this.name,
			SharefName: name,
			Previous:   previous,
			Current:    current,
		}
		this.onReadWrite(event)
	}
}
