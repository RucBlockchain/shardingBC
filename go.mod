module github.com/tendermint/tendermint

go 1.13

require (
	github.com/btcsuite/btcd v0.20.1-beta
	github.com/btcsuite/btcutil v0.0.0-20190425235716-9e5f4b9a998d
	github.com/btcsuite/websocket v0.0.0-20150119174127-31079b680792
	github.com/coreos/etcd v3.3.25+incompatible
	github.com/dustin/go-humanize v1.0.0 // indirect
	github.com/fortytw2/leaktest v1.3.0
	github.com/go-kit/kit v0.10.0
	github.com/go-logfmt/logfmt v0.5.0
	github.com/gogo/protobuf v1.3.1
	github.com/golang/protobuf v1.4.2
	github.com/google/uuid v1.1.2 // indirect
	github.com/gorilla/websocket v1.4.2
	github.com/jmhodges/levigo v1.0.0
	github.com/magiconair/properties v1.8.2
	github.com/mitchellh/mapstructure v1.3.3 // indirect
	github.com/pelletier/go-toml v1.8.1 // indirect
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.7.1
	github.com/rcrowley/go-metrics v0.0.0-20200313005456-10cdbea86bc0
	github.com/rs/cors v1.7.0
	github.com/spf13/afero v1.3.5 // indirect
	github.com/spf13/cast v1.3.1 // indirect
	github.com/spf13/cobra v1.0.0
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/spf13/viper v1.7.1
	github.com/stretchr/testify v1.4.0
	github.com/syndtr/goleveldb v1.0.0
	github.com/tendermint/go-amino v0.15.1
	github.com/tendermint/tmlibs v0.9.0
	go.dedis.ch/kyber/v3 v3.0.12
	go.etcd.io/etcd v0.0.0-20191023171146-3cf2f69b5738
	golang.org/x/crypto v0.0.0-20200820211705-5c72a883971a
	golang.org/x/exp v0.0.0-20200908183739-ae8ad444f925
	golang.org/x/net v0.0.0-20200904194848-62affa334b73
	golang.org/x/sys v0.0.0-20200625212154-ddb9806d33ae // indirect
	google.golang.org/genproto v0.0.0-20200702021140-07506425bd67 // indirect
	google.golang.org/grpc v1.30.0
	gopkg.in/ini.v1 v1.61.0 // indirect
	sigs.k8s.io/yaml v1.2.0 // indirect
)

replace go.etcd.io/etcd => github.com/etcd-io/etcd v3.3.10+incompatible

replace google.golang.org/grpc => google.golang.org/grpc v1.26.0
