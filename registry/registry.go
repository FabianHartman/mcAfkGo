package registry

type Registry[E any] struct {
	keys    map[string]int32
	values  []E
	indices map[*E]int32
	tags    map[string][]*E
}

func (r *Registry[E]) Put(key string, data E) (id int32, val *E) {
	id = int32(len(r.values))
	r.keys[key] = id
	r.values = append(r.values, data)
	val = &r.values[id]
	r.indices[val] = id
	return
}
