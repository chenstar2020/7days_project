package codec

import "io"

type Header struct {
	ServiceMethod string  //服务名和方法名 format "Service.Method"
	Seq uint64            //序列号 sequence number chosen by client
	Error string
}

type Codec interface {
	io.Closer
	ReadHeader(*Header)error
	ReadBody(interface{})error
	Write(*Header, interface{})error
}


type NewCodecFunc func(closer io.ReadWriteCloser)Codec

type Type string

const(
	GobType Type = "application/gob"
	JsonType Type = "application/json"  //not implemented
)

var NewCodecFuncMap map[Type]NewCodecFunc

func init(){
	NewCodecFuncMap = make(map[Type]NewCodecFunc)
	NewCodecFuncMap[GobType] = NewGobCodec
}