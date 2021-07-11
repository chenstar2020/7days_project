package xclient

import (
	"errors"
	"math"
	"math/rand"
	"sync"
	"time"
)

type SelectMode int

const(
	RandomSelect SelectMode = iota       //随机选择
	RoundRobinSelect                     //轮询选择
)

type Discovery interface {
	Refresh() error
	Update(servers []string)error
	Get(mode SelectMode)(string, error)
	GetAll()([]string, error)
}

type MultiServersDiscovery struct{
	r *rand.Rand
	mu sync.RWMutex
	servers []string
	index int            //index记录算法轮询到的位置
}



func NewMultiServerDiscovery(servers []string)*MultiServersDiscovery{
	d := &MultiServersDiscovery{
		servers: servers,
		r: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	d.index = d.r.Intn(math.MaxInt32 - 1)  //生成随机值，防止每次从0开始轮询
	return d
}

var _ Discovery = (*MultiServersDiscovery)(nil)

func (d *MultiServersDiscovery)Refresh()error{
	return nil
}

func (d *MultiServersDiscovery)Update(servers []string)error{
	d.mu.Lock()
	defer d.mu.Unlock()
	d.servers = servers

	return nil
}

func (d *MultiServersDiscovery) Get(mode SelectMode) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	n := len(d.servers)
	if n == 0 {
		return "", errors.New("rpc discovery: no available servers")
	}
	switch mode{
	case RandomSelect:        //随机选择
		return d.servers[d.r.Intn(n)], nil
	case RoundRobinSelect:    //轮询选择
		s := d.servers[d.index % n]
		d.index = (d.index + 1) % n
		return s, nil
	default:
		return "", errors.New("rpc discovery: not supported select mode")
	}
}

func (d *MultiServersDiscovery) GetAll() ([]string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	servers := make([]string, len(d.servers), len(d.servers))
	copy(servers, d.servers)
	return servers, nil
}


