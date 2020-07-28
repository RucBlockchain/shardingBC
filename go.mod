module github.com/tendermint/tendermint

go 1.13

require (
	github.com/btcsuite/btcd v0.20.1-beta
	github.com/coreos/etcd v3.3.22+incompatible
	github.com/go-kit/kit v0.10.0
	github.com/go-logfmt/logfmt v0.5.0
	github.com/gogo/protobuf v1.3.1
	github.com/golang/protobuf v1.4.2
	github.com/gorilla/websocket v1.4.2
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.7.1
	github.com/rcrowley/go-metrics v0.0.0-20200313005456-10cdbea86bc0
	github.com/rs/cors v1.7.0
	github.com/spf13/cobra v1.0.0
	github.com/spf13/viper v1.7.0
	github.com/stretchr/testify v1.4.0
	github.com/syndtr/goleveldb v1.0.0
	github.com/tendermint/go-amino v0.15.1
	go.dedis.ch/kyber/v3 v3.0.12
	golang.org/x/crypto v0.0.0-20200622213623-75b288015ac9
	golang.org/x/exp v0.0.0-20191030013958-a1ab85dbe136
	golang.org/x/net v0.0.0-20200625001655-4c5254603344
	golang.org/x/sys v0.0.0-20200625212154-ddb9806d33ae // indirect
	google.golang.org/genproto v0.0.0-20200702021140-07506425bd67 // indirect
	google.golang.org/grpc v1.30.0
)

replace go.etcd.io/etcd => github.com/etcd-io/etcd v3.3.10+incompatible

replace google.golang.org/grpc => google.golang.org/grpc v1.26.0
