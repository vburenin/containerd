package vicruntime

import (
	"sync"

	"github.com/containerd/containerd/plapi/client"
)

var plonce sync.Once
var plClient *client.PortLayer

func PortLayerClient(addr string) *client.PortLayer {
	plonce.Do(func() {
		cfg := client.DefaultTransportConfig().WithHost(addr)
		plClient = client.NewHTTPClientWithConfig(nil, cfg)
	})
	return plClient
}
