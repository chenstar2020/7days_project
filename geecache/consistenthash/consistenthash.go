package consistenthash

import (
	"hash/crc32"
	"sort"
	"strconv"
)

//Hash maps bytes to uint32
type Hash func(data []byte) uint32


//Map constains all hashed keys
type Map struct {
	hash Hash
	replicas int    //虚拟节点倍数
	keys []int
	hashMap map[int]string
}

func New(replicas int, fn Hash) *Map {
	m := &Map{
		replicas: replicas,
		hash: fn,
		hashMap: make(map[int]string),
	}

	if m.hash == nil {
		m.hash = crc32.ChecksumIEEE
	}

	return m
}

//添加节点hash以及虚拟节点hash
func (m *Map)Add(keys ...string){
	for _, key := range keys {
		for i := 0; i < m.replicas; i++ {
			hash := int(m.hash([]byte(strconv.Itoa(i) + key)))
			m.keys = append(m.keys, hash)
			m.hashMap[hash] = key
		}
	}
	sort.Ints(m.keys)
}

//删除节点 TODO 未测试
func (m *Map)Del(key string){
	for i := 0; i < m.replicas; i++ {
		hash := int(m.hash([]byte(strconv.Itoa(i) + key)))

		for j := 0; j < len(m.keys); j++ {
			if m.keys[j] == hash {
				m.keys = append(m.keys[:j], m.keys[j + 1:]...)
				break
			}
		}

		delete(m.hashMap, hash)
	}
}


//得到key对应的节点
func (m *Map)Get(key string) string {
	if len(m.keys) == 0 {
		return ""
	}

	hash := int(m.hash([]byte(key)))
	idx := sort.Search(len(m.keys), func(i int)bool{
		return m.keys[i] >= hash
	})

	return m.hashMap[m.keys[idx % len(m.keys)]]
}