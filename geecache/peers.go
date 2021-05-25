package geecache

import "geecache/geecachepb"

type PeerPicker interface{
	PickPeer(key string)(peer PeerGetter, ok bool)
}

type PeerGetter interface{
	//Get(group string, key string)([]byte, error)
	Get(in *geecachepb.Request, out *geecachepb.Response)error
}
