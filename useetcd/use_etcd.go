package use_etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/clientv3/concurrency"
	"log"
	"time"
)

type Use_Etcd struct {

	Endpoints []string
}

func (e Use_Etcd)Delete(key string,value string)  {
	//获取到分布式锁
	//删除数据
	cli, err := clientv3.New(clientv3.Config{Endpoints: e.Endpoints, DialTimeout: 5 * time.Second})
	defer cli.Close()
	if err!=nil{
		fmt.Println("conn failure!")
		return
	}
	if err != nil {
		log.Fatal(err)
	}
	ss, err := concurrency.NewSession(cli, concurrency.WithContext(context.Background()))
	shard:=key+"lock"
	mu := concurrency.NewMutex(ss, shard)
	fmt.Println("try get Delete lock ... ")
	err = mu.Lock(context.Background())
	if err != nil {
		fmt.Println("get "+key+"lock failed!")
		return
	}
	fmt.Println("Delete lock "+key+" success")//获取到锁
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	shard_key:=key+"list"
	//对value进行json格式添加打包。先获取原本的内容，再进行添加
	//获取内容
	resp, err := cli.Get(ctx,shard_key)
	var content []byte
	for _, ev := range resp.Kvs {
		content=ev.Value
	}
	var m []string

	json.Unmarshal(content,&m)
	//fmt.Println(m)
	for i:=0;i<len(m);i++{
		if m[i]==value{
			m = append(m[:i],m[i+1:]...)
		}
	}
	//fmt.Println(m)
	res1, _ := json.Marshal(m)
	rawParamsJSON1 := json.RawMessage(res1)
	cli.Put(ctx, shard_key,string(rawParamsJSON1))//传输完成
	fmt.Println("删除了节点",value)
	cancel()
	err = mu.Unlock(context.Background())//解锁
	if err != nil {
		fmt.Println("free Delete lock "+key+"failed!")
		return
	}
	fmt.Println("free Delete lock "+key+" succeed!")
}
func (e Use_Etcd)Save(key string,value string){
	//这个方法：1.获取分布式写锁。
	//		  2.写数据
	cli, err := clientv3.New(clientv3.Config{Endpoints: e.Endpoints, DialTimeout: 5 * time.Second})
	defer cli.Close()
	if err!=nil{
		fmt.Println("conn failure!")
		return
	}
	if err != nil {
		log.Fatal(err)
	}
	ss, err := concurrency.NewSession(cli, concurrency.WithContext(context.Background()))
	shard:=key+"lock"
	mu := concurrency.NewMutex(ss, shard)
	fmt.Println("try get Save lock ... ")
	err = mu.Lock(context.Background())
	if err != nil {
		fmt.Println("get "+key+" Save lock failed!")
		return
	}
	fmt.Println("Save lock "+key+" success")//获取到锁
	//进行操作
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	shard_key:=key+"list"
	//对value进行json格式添加打包。先获取原本的内容，再进行添加
	//获取内容
	resp, err := cli.Get(ctx,shard_key)
	var content []byte
	for _, ev := range resp.Kvs {
		content=ev.Value
	}
	//获取到内容,对内容进行解析
	//解析并添加元素，并且存储到etcd
	var m []string
	json.Unmarshal(content,&m)
	m=append(m, value)
	//fmt.Println(m)
	res1, _ := json.Marshal(m)
	rawParamsJSON1 := json.RawMessage(res1)
	cli.Put(ctx, shard_key,string(rawParamsJSON1))
	fmt.Println("添加了节点",value)
	cancel()
	err = mu.Unlock(context.Background())//解锁
	if err != nil {
		fmt.Println("free Save lock "+key+" failed!")
		return
	}
	fmt.Println("free Save lock "+key+"succeed!")
}
func (e Use_Etcd)Update(key string,value string){
	cli, err := clientv3.New(clientv3.Config{
	Endpoints:e.Endpoints,
	DialKeepAliveTime:5*time.Second,
	})
	Value :=""
	Value = value+":26657"

	key = "/"+key
	if err!=nil{
	fmt.Println("conn failure!")
	}
	defer cli.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	_, err = cli.Put(ctx, key, Value)
	cancel()
	if err!=nil{
		fmt.Println("put failed!")
		return
	}
	fmt.Println("put success!")
	return
}
func NewEtcd()(*Use_Etcd){
	return &Use_Etcd{
		Endpoints: []string{"etcd1:2379"},
	}
}

func (e Use_Etcd)Query(key string)(value []byte){
	cli,err := clientv3.New(clientv3.Config{
		Endpoints:e.Endpoints,
		DialKeepAliveTime:5*time.Second,
	})
	key = "/"+key	
	//fmt.Println(key)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	resp, err := cli.Get(ctx,key)
	cancel()
	if err != nil {
		fmt.Println("get failed, err:", err)
		return
	}
	for _, ev := range resp.Kvs {
		b:=ev.Value
		//fmt.Printf("%s : %s\n", ev.Key, ev.Value)
		return b
	}
	return
}
