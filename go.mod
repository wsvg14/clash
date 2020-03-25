module github.com/Dreamacro/clash

go 1.14

require (
	github.com/Dreamacro/go-shadowsocks2 v0.1.5
	github.com/eapache/queue v1.1.0 // indirect
	github.com/go-chi/chi v4.0.3+incompatible
	github.com/go-chi/cors v1.0.1
	github.com/go-chi/render v1.0.1
	github.com/gofrs/uuid v3.2.0+incompatible
	github.com/gorilla/websocket v1.4.2
	github.com/miekg/dns v1.1.29
	github.com/oschwald/geoip2-golang v1.4.0
	github.com/sirupsen/logrus v1.4.2
	github.com/stretchr/testify v1.5.1
	golang.org/x/crypto v0.0.0-20200320181102-891825fb96df
	golang.org/x/net v0.0.0-20200320220750-118fecf932d8
	golang.org/x/sync v0.0.0-20200317015054-43a5402ce75a
	golang.org/x/sys v0.0.0-20200302150141-5c8b2ff67527
	gopkg.in/eapache/channels.v1 v1.1.0
	gopkg.in/yaml.v2 v2.2.8
	gvisor.dev/gvisor v0.0.0-00010101000000-000000000000
)

replace gvisor.dev/gvisor => github.com/comzyh/gvisor v0.0.0-20200420094407-0d4ba3bd3efa
