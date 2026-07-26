package nbt

import (
	"reflect"
	"sort"
	"strings"
	"sync"
)

type structFields struct {
	list      []field
	nameIndex map[string]int
}

type field struct {
	name string

	tag       bool
	index     []int
	typ       reflect.Type
	omitEmpty bool
	asList    bool
}

type byIndex []field

func (x byIndex) Len() int { return len(x) }

func (x byIndex) Swap(i, j int) { x[i], x[j] = x[j], x[i] }

func (x byIndex) Less(i, j int) bool {
	for k, xik := range x[i].index {
		if k >= len(x[j].index) {
			return false
		}

		if xik != x[j].index[k] {
			return xik < x[j].index[k]
		}

	}

	return len(x[i].index) < len(x[j].index)
}

func typeFields(t reflect.Type) (tInfo structFields) {
	current := []field{}
	next := []field{{typ: t}}

	var count, nextCount map[reflect.Type]int

	visited := make(map[reflect.Type]struct{})

	var fields []field

	for len(next) > 0 {
		current, next = next, current[:0]
		count, nextCount = nextCount, make(map[reflect.Type]int)

		for _, f := range current {
			_, ok := visited[f.typ]
			if ok {
				continue
			}
			visited[f.typ] = struct{}{}

			for i := 0; i < f.typ.NumField(); i++ {
				sf := f.typ.Field(i)
				if sf.Anonymous {
					t := sf.Type
					if t.Kind() == reflect.Pointer {
						t = t.Elem()
					}

					if !sf.IsExported() && t.Kind() != reflect.Struct {
						continue
					}
				} else if !sf.IsExported() {
					continue
				}

				tag := sf.Tag.Get("nbt")
				if tag == "-" {
					continue
				}

				name, opts, _ := strings.Cut(tag, ",")
				index := make([]int, len(f.index)+1)
				copy(index, f.index)
				index[len(f.index)] = i
				keytag := sf.Tag.Get("nbtkey")
				if keytag != "" {
					name = keytag
				}

				ft := sf.Type
				if ft.Name() == "" && ft.Kind() == reflect.Pointer {
					ft = ft.Elem()
				}

				var omitEmpty, asList bool
				for opts != "" {
					var name string
					name, opts, _ = strings.Cut(opts, ",")
					switch name {
					case "omitempty":
						omitEmpty = true
					case "list":
						asList = true
					}
				}

				if sf.Tag.Get("nbt_type") == "list" {
					asList = true
				}

				if name != "" || !sf.Anonymous || ft.Kind() != reflect.Struct {
					tagged := name != ""
					if name == "" {
						name = sf.Name
					}

					field := field{
						name:      name,
						tag:       tagged,
						index:     index,
						typ:       ft,
						omitEmpty: omitEmpty,
						asList:    asList,
					}

					fields = append(fields, field)
					if count[f.typ] > 1 {
						fields = append(fields, fields[len(fields)-1])
					}

					continue
				}

				nextCount[ft]++
				if nextCount[ft] == 1 {
					next = append(next, field{name: ft.Name(), index: index, typ: ft})
				}
			}
		}
	}

	sort.Slice(fields, func(i, j int) bool {
		x := fields
		if x[i].name != x[j].name {
			return x[i].name < x[j].name
		}

		if len(x[i].index) != len(x[j].index) {
			return len(x[i].index) < len(x[j].index)
		}

		if x[i].tag != x[j].tag {
			return x[i].tag
		}

		return byIndex(x).Less(i, j)
	})

	out := fields[:0]
	for advance, i := 0, 0; i < len(fields); i += advance {
		fi := fields[i]
		name := fi.name
		for advance = 1; i+advance < len(fields); advance++ {
			fj := fields[i+advance]
			if fj.name != name {
				break
			}
		}

		if advance == 1 {
			out = append(out, fi)

			continue
		}

		dominant, ok := dominantField(fields[i : i+advance])
		if ok {
			out = append(out, dominant)
		}
	}

	fields = out
	sort.Sort(byIndex(fields))

	nameIndex := make(map[string]int, len(fields))
	for i, field := range fields {
		nameIndex[field.name] = i
	}

	return structFields{
		list:      fields,
		nameIndex: nameIndex,
	}
}

func dominantField(fields []field) (field, bool) {
	if len(fields) > 1 && len(fields[0].index) == len(fields[1].index) && fields[0].tag == fields[1].tag {
		return field{}, false
	}

	return fields[0], true
}

var fieldCache sync.Map

func cachedTypeFields(t reflect.Type) structFields {
	if ti, ok := fieldCache.Load(t); ok {
		return ti.(structFields)
	}

	tInfo := typeFields(t)
	ti, _ := fieldCache.LoadOrStore(t, tInfo)

	return ti.(structFields)
}
