package common

import (
	"geerpc/codec"
	"time"
)

const MagicNumber = 0x3bef5c

type Option struct {
	MagicNumber int  // MagicNumber marks this's a geerpc request  使用魔数标记请求类型
	CodecType codec.Type
	ConnectTimeout time.Duration //0 means no limit
	HandleTimeout time.Duration
}

var DefaultOption = &Option{
	MagicNumber: MagicNumber,
	CodecType: codec.GobType,
	ConnectTimeout: time.Second *10,
}


const (
	Connected = "200 Connected to Gee RPC"
	DefaultRPCPath = "/_geeprc_"
	DefaultDebugPath = "/debug/geerpc"
)
