
all:
	go build -buildmode=plugin -o vicshim-linux-amd64.so

install:
	mkdir -p /var/lib/containerd/plugins
	cp vicshim-linux-amd64.so /var/lib/containerd/plugins/
