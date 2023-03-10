package httpadapter

type Option[T any] interface {
	apply(*T)
}

func NewOption[T any](f func(*T)) Option[T] {
	return newFuncOptions(f)
}

type funcOptions[T any] struct {
	f func(*T)
}

func newFuncOptions[T any](f func(*T)) *funcOptions[T] {
	return &funcOptions[T]{
		f: f,
	}
}
func (o *funcOptions[T]) apply(opts *T) {
	o.f(opts)
}
