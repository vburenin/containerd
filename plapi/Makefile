
all:
	go build -buildmode=plugin -o vic-linux-amd64.so

install:
	mkdir -p /var/lib/containerd/plugins
	cp vic-linux-amd64.so /var/lib/containerd/plugins/
