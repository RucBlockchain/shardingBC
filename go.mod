module github.com/tendermint/tendermint

go 1.12

require (
	github.com/beorn7/perks v0.0.0-20180321164747-3a771d992973
	github.com/btcsuite/btcd v0.0.0-20190115013929-ed77733ec07d
	github.com/btcsuite/btcutil v0.0.0-20180706230648-ab6388e0c60a
	github.com/coreos/etcd v3.3.18+incompatible // indirect
	github.com/coreos/go-systemd v0.0.0-00010101000000-000000000000 // indirect
	github.com/coreos/pkg v0.0.0-20180928190104-399ea9e2e55f // indirect
	github.com/davecgh/go-spew v1.1.1
	github.com/fortytw2/leaktest v0.0.0-20180314164940-a5ef70473c97
	github.com/fsnotify/fsnotify v1.4.7
	github.com/go-kit/kit v0.0.0-20171003134444-4dc7be5d2d12
	github.com/go-logfmt/logfmt v0.0.0-20161115142513-390ab7935ee2
	github.com/go-stack/stack v0.0.0-20180826134848-2fee6af1a979
	github.com/gogo/protobuf v0.0.0-20190218063003-ba06b47c162d
	github.com/golang/protobuf v1.3.2
	github.com/golang/snappy v0.0.0-20180518054509-2e65f85255db
	github.com/google/uuid v1.1.1 // indirect
	github.com/gorilla/websocket v0.0.0-20170620190103-ea4d1f681bab
	github.com/hashicorp/hcl v0.0.0-20180826005136-8cb6e5b95923
	github.com/inconshreveable/mousetrap v0.0.0-20141017200713-76626ae9c91c
	github.com/jmhodges/levigo v0.0.0-20190228103307-853d788c5c41
	github.com/kr/logfmt v0.0.0-20140226030751-b84e30acd515
	github.com/magiconair/properties v0.0.0-20180515204034-c2353362d570
	github.com/matttproud/golang_protobuf_extensions v0.0.0-20160424113007-c12348ce28de
	github.com/mitchellh/mapstructure v0.0.0-20181005045135-3536a929eddd
	github.com/pelletier/go-toml v0.0.0-20180605204719-c01d1270ff3e
	github.com/pkg/errors v0.8.1
	github.com/pmezard/go-difflib v1.0.0
	github.com/prometheus/client_golang v0.0.0-20181103142328-abad2d1bd442
	github.com/prometheus/client_model v0.0.0-20190812154241-14fe0d1b01d4
	github.com/prometheus/common v0.0.0-20181020173914-7e9e6cabbd39
	github.com/prometheus/procfs v0.0.0-20181005140218-185b4288413d
	github.com/rcrowley/go-metrics v0.0.0-20180503174638-e2704e165165
	github.com/rs/cors v0.0.0-20181001164945-9a47f48565a7
	github.com/spf13/afero v0.0.0-20180907092351-d40851caa0d7
	github.com/spf13/cast v0.0.0-20181024225928-8c9545af88b1
	github.com/spf13/cobra v0.0.0-20171012182533-7b2c5ac9fc04
	github.com/spf13/jwalterweatherman v0.0.0-20180907091759-4a4406e478ca
	github.com/spf13/pflag v0.0.0-20180831151432-298182f68c66
	github.com/spf13/viper v0.0.0-20170723055207-25b30aa063fc
	github.com/stretchr/testify v1.4.0
	github.com/syndtr/goleveldb v0.0.0-20181012014443-6b91fda63f2e
	github.com/tendermint/go-amino v0.0.0-20181111211044-dc14acf9ef15
	go.etcd.io/etcd v3.3.18+incompatible
	go.uber.org/zap v1.13.0 // indirect
	golang.org/x/crypto v0.0.0-20190510104115-cbcb75029529
	golang.org/x/net v0.0.0-20190620200207-3b0461eec859
	google.golang.org/grpc v1.26.0
	gopkg.in/yaml.v2 v2.2.2
)

replace github.com/coreos/go-systemd => github.com/coreos/go-systemd/v22 v22.0.0
