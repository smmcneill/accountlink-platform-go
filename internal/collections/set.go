package collections

type Set[T comparable] struct {
	values map[T]struct{}
}

func NewSet[T comparable]() *Set[T] {
	return &Set[T]{values: make(map[T]struct{})}
}

func (s *Set[T]) Add(value T) {
	s.values[value] = struct{}{}
}

func (s *Set[T]) Has(value T) bool {
	_, ok := s.values[value]
	return ok
}
