package sonic

func GetLowestLayer[T any](layer Layered[T]) T {
	for {
		next, ok := any(layer.NextLayer()).(Layered[T])
		if ok {
			layer = next
		} else {
			return layer.NextLayer()
		}
	}
}
