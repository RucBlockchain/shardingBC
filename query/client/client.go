package client

import (
	"context"
	"fmt"
	"go.etcd.io/etcd/clientv3"
	"time"
)

func NewEtcd() *Use_Etcd {
	return &Use_Etcd{
		Endpoints: []string{"0.0.0.0:2379"},
	}
}

type Use_Etcd struct {
	Endpoints []string
}

func (e Use_Etcd) Update(key string, value string) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:         e.Endpoints,
		DialKeepAliveTime: 5 * time.Second,
	})
	Value := ""
	Value = value + ":26657"

	key = "/" + key
	if err != nil {
		fmt.Println("conn failure!")
	}
	defer cli.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	_, err = cli.Put(ctx, key, Value)
	cancel()
	if err != nil {
		fmt.Println("put failed!")
		return
	}
	fmt.Println("put success!")
	return
}
func (e Use_Etcd) Query(key string) (value []byte) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:         e.Endpoints,
		DialKeepAliveTime: 5 * time.Second,
	})
	key = "/" + key
	//fmt.Println(key)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	resp, err := cli.Get(ctx, key)
	cancel()
	if err != nil {
		fmt.Println("get failed, err:", err)
		return
	}
	for _, ev := range resp.Kvs {
		b := ev.Value
		fmt.Printf("%s : %s\n", ev.Key, ev.Value)
		return b
	}
	return
}
