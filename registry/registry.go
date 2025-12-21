package registry

type Registry[E any] struct {
	keys    map[string]int32
	values  []E
	indices map[*E]int32
	tags    map[string][]*E
}

func NewRegistry[E any]() Registry[E] {
	return Registry[E]{
		keys:    make(map[string]int32),
		values:  make([]E, 0, 256),
		indices: make(map[*E]int32),
		tags:    make(map[string][]*E),
	}
}

func (r *Registry[E]) Clear() {
	r.keys = make(map[string]int32)
	r.values = r.values[:0]
	r.indices = make(map[*E]int32)
	r.tags = make(map[string][]*E)
}

func (r *Registry[E]) Put(key string, data E) (id int32, val *E) {
	id = int32(len(r.values))
	r.keys[key] = id
	r.values = append(r.values, data)
	val = &r.values[id]
	r.indices[val] = id
	return
}
