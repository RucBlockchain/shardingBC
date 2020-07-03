module github.com/tendermint/tendermint

go 1.13

require (
	github.com/VividCortex/gohistogram v1.0.0 // indirect
	github.com/beorn7/perks v0.0.0-20180321164747-3a771d992973 // indirect
	github.com/btcsuite/btcd v0.0.0-20190115013929-ed77733ec07d
	github.com/btcsuite/btcutil v0.0.0-20180706230648-ab6388e0c60a
	github.com/coreos/bbolt v1.3.3 // indirect
	github.com/coreos/etcd v3.3.18+incompatible
	github.com/coreos/go-semver v0.3.0 // indirect
	github.com/coreos/go-systemd v0.0.0-00010101000000-000000000000 // indirect
	github.com/coreos/pkg v0.0.0-20180928190104-399ea9e2e55f // indirect
	github.com/dgrijalva/jwt-go v3.2.0+incompatible // indirect
	github.com/fortytw2/leaktest v0.0.0-20180314164940-a5ef70473c97
	github.com/go-kit/kit v0.8.0
	github.com/go-logfmt/logfmt v0.3.0
	github.com/go-stack/stack v1.8.0 // indirect
	github.com/gogo/protobuf v1.2.1
	github.com/golang/groupcache v0.0.0-20191227052852-215e87163ea7 // indirect
	github.com/golang/protobuf v1.3.2
	github.com/google/btree v1.0.0 // indirect
	github.com/google/go-cmp v0.3.1 // indirect
	github.com/google/uuid v1.1.1 // indirect
	github.com/gorilla/websocket v1.4.1-0.20190629185528-ae1634f6a989
	github.com/grpc-ecosystem/go-grpc-middleware v1.1.0 // indirect
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.12.1 // indirect
	github.com/hashicorp/hcl v0.0.0-20180826005136-8cb6e5b95923 // indirect
	github.com/inconshreveable/mousetrap v0.0.0-20141017200713-76626ae9c91c // indirect
	github.com/jmhodges/levigo v0.0.0-20190228103307-853d788c5c41
	github.com/jonboulle/clockwork v0.1.0 // indirect
	github.com/json-iterator/go v1.1.9 // indirect
	github.com/kr/logfmt v0.0.0-20140226030751-b84e30acd515 // indirect
	github.com/magiconair/properties v0.0.0-20180515204034-c2353362d570
	github.com/matttproud/golang_protobuf_extensions v1.0.1 // indirect
	github.com/mitchellh/mapstructure v0.0.0-20181005045135-3536a929eddd // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.1 // indirect
	github.com/pelletier/go-toml v0.0.0-20180605204719-c01d1270ff3e // indirect
	github.com/pkg/errors v0.8.1
	github.com/prometheus/client_golang v0.9.1
	github.com/prometheus/common v0.0.0-20181113130724-41aa239b4cce // indirect
	github.com/prometheus/procfs v0.0.0-20181005140218-185b4288413d // indirect
	github.com/rcrowley/go-metrics v0.0.0-20180503174638-e2704e165165
	github.com/rs/cors v0.0.0-20181001164945-9a47f48565a7
	github.com/soheilhy/cmux v0.1.4 // indirect
	github.com/spf13/afero v1.2.2 // indirect
	github.com/spf13/cast v0.0.0-20181024225928-8c9545af88b1 // indirect
	github.com/spf13/cobra v0.0.0-20171012182533-7b2c5ac9fc04
	github.com/spf13/jwalterweatherman v0.0.0-20180907091759-4a4406e478ca // indirect
	github.com/spf13/pflag v1.0.1 // indirect
	github.com/spf13/viper v0.0.0-20170723055207-25b30aa063fc
	github.com/stretchr/testify v1.4.0
	github.com/syndtr/goleveldb v1.0.1-0.20190923125748-758128399b1d
	github.com/tendermint/go-amino v0.0.0-20181111211044-dc14acf9ef15
	github.com/tendermint/tmlibs v0.9.0
	github.com/tmc/grpc-websocket-proxy v0.0.0-20190109142713-0ad062ec5ee5 // indirect
	github.com/xiang90/probing v0.0.0-20190116061207-43a291ad63a2 // indirect
	go.dedis.ch/kyber/v3 v3.0.10
	go.etcd.io/bbolt v1.3.3 // indirect
	go.etcd.io/etcd v0.0.0-00010101000000-000000000000
	go.uber.org/zap v1.13.0 // indirect
	golang.org/x/crypto v0.0.0-20200311171314-f7b00557c8c4
	golang.org/x/net v0.0.0-20200301022130-244492dfa37a
	golang.org/x/sys v0.0.0-20200302150141-5c8b2ff67527 // indirect
	golang.org/x/text v0.3.2 // indirect
	golang.org/x/time v0.0.0-20191024005414-555d28b269f0 // indirect
	google.golang.org/grpc v1.26.0
	sigs.k8s.io/yaml v1.1.0 // indirect
)

replace github.com/coreos/go-systemd => github.com/coreos/go-systemd/v22 v22.0.0

replace go.etcd.io/etcd => github.com/etcd-io/etcd v3.3.10+incompatible

replace gopkg.in/urfave/cli.v2 => github.com/urfave/cli v1.22.3

replace go.dedis.ch/kyber/v3 v3.0.10 => github.com/dedis/kyber/v3 v3.0.10
