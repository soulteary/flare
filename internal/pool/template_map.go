package pool

import "sync"

const templateMapCap = 48

var templateMapPool = sync.Pool{
	New: func() any { return make(map[string]interface{}, templateMapCap) },
}

// GetTemplateMap 从池中取一个 map，用完后须调用 PutTemplateMap 归还。
func GetTemplateMap() map[string]interface{} {
	m, ok := templateMapPool.Get().(map[string]interface{})
	if !ok {
		return make(map[string]interface{}, templateMapCap)
	}
	return m
}

// PutTemplateMap 清空 map 并归还到池。
func PutTemplateMap(m map[string]interface{}) {
	for k := range m {
		delete(m, k)
	}
	templateMapPool.Put(m)
}
