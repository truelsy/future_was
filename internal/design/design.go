package design

// Design 디자인 도메인 1개의 데이터를 PK로 인덱싱한 읽기 전용 컬렉션이다.
// Loader가 set으로 채운 뒤, 사용처에서는 Get/All만 호출한다.
type Design[K comparable, V any] struct {
	m map[K]V
}

func NewDesign[K comparable, V any]() *Design[K, V] {
	return &Design[K, V]{m: map[K]V{}}
}

// Get PK에 해당하는 항목을 반환한다. 없으면 V의 zero value (포인터면 nil).
func (t *Design[K, V]) Get(k K) V {
	return t.m[k]
}

// Find Get + 존재 여부.
func (t *Design[K, V]) Find(k K) (V, bool) {
	v, ok := t.m[k]
	return v, ok
}

// All 내부 map을 반환한다 (수정 금지, iteration 전용).
func (t *Design[K, V]) All() map[K]V {
	return t.m
}

// Len 항목 수.
func (t *Design[K, V]) Len() int {
	return len(t.m)
}

// set Loader 전용. 외부에서는 호출하지 않는다.
func (t *Design[K, V]) set(k K, v V) {
	t.m[k] = v
}
