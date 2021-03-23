package Line

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	rpctypes "github.com/tendermint/tendermint/rpc/lib/types"
	useetcd "github.com/tendermint/tendermint/useetcd"
)

var Flag_conn map[string][]bool
var Shard int
var endpoints node
var wg sync.WaitGroup
var Count = 0
var Shard_name = ""
var Connect_success = false //等待初始化完成才能使用
var sendTimeout = 11 * time.Second
var pingPeriod = 11 * time.Second
var Send_flag []bool                        //设定定时器，达到一定时间发送交易
var timer = time.NewTicker(time.Second * 5) //周期5s
var Judge_leader = false                    //判断是否是leader，只有leader才能建立连接
//var Count map[int][]int
//func Count_int(){
//	Count=make(map[int][]int,4)
//	for i:=0;i<4;i++{
//		Count[i]=make([]int,10)
//		for j:=0;j<10;j++{
//			Count[i][j]=0
//		}
//	}
//}
func time1() { //定时器实现
	for {
		select {
		case <-timer.C:
			for i := range Send_flag {
				Send_flag[i] = false
			}
		}
	}
}
func Flag_init() { //初始化链接没使用则为false
	Flag_conn = make(map[string][]bool, Shard) //初始设置n个分片
	for i := 0; i < Shard+1; i++ {             //取到所有的连接，在初始化一个本地
		if i == Shard {
			Flag_conn["Localhost"] = make([]bool, 10)
			Flag_conn["Localhost"][0] = false
			break
		}

		name := string(i + 65)
		if name == Shard_name {
			continue
		}
		//fmt.Println("初始化效果",name)
		Flag_conn[name] = make([]bool, 10)
		for j := 0; j < 10; j++ {
			Flag_conn[name][j] = false
		}
	}
}
func keep_message(shard string, i int) {
	if Flag_conn[shard][i] == false {
		Flag_conn[shard][i] = true
		c := l.conns[shard][i]
		c.SetWriteDeadline(time.Now().Add(sendTimeout))

		if err := c.WriteMessage(websocket.PongMessage, []byte{}); err != nil {
			fmt.Println("守护进程出错", err)
			c, _, err1 := Connect(l.target[shard][i])
			if err1 != nil {
				fmt.Println(err1)
				return
			}
			l.conns[shard][i] = c
			fmt.Println("重新连接")
		}
		//fmt.Println("连接畅通")
		Flag_conn[shard][i] = false
	}
}
func (l *Line) KeepAlive() {
	pingsTicker := time.NewTicker(pingPeriod)
	for {
		select {
		case <-pingsTicker.C:
			//fmt.Println("定时器")
			for shard := range l.target {
				for i, _ := range l.target[shard] {
					//fmt.Println("连接",i)
					keep_message(shard, i)
				}

			}
			fmt.Println("维护连接")

		}
	}
}
func Shard_init() {
	Shard = 0
}
func judge_etcd(e *useetcd.Use_Etcd, i int) {
	var ip string
	for {

		ip = string(e.Query(string(i + 65)))
		if ip == "" {
			fmt.Println("睡觉～～～")

			time.Sleep(time.Second * 2)
			continue
		} else {
			fmt.Println("ip=", ip)
			break
		}
	}
	for j := 0; j < 10; j++ {
		endpoints.target[string(i+65)] = append(endpoints.target[string(i+65)], ip)
	}
	defer wg.Done()
	return
}
func newline() *Line {

	endpoints.target = make(map[string][]string, Shard)
	e := useetcd.NewEtcd()

	wg.Add(Shard)
	for i := 0; i < Shard; i++ {
		name := string(i + 65)
		if Shard_name == name {
			wg.Done()
			continue
		}
		judge_etcd(e, i)
	}

	Name := "tt" + Shard_name + "node1:26657"
	//fmt.Println(Name)
	endpoints.target["Localhost"] = []string{Name}
	wg.Wait()
	fmt.Println(endpoints.target)
	fmt.Println("初始化完成")
	l1 := NewLine(endpoints.target)
	return l1
}

type Line struct {
	target map[string][]string
	conns  map[string][]*websocket.Conn
}

//type Cn struct {
//	conn *websocket.Conn
//	mu sync.Mutex
//}
type node struct {
	target map[string][]string
}

var l *Line

//var cn1 *Cn
func begin() {
	if err := l.Start(); err != nil {
		fmt.Println(err)
		return
	}

	return
}
func figure_Shard() {
	for {
		if Shard == 0 || Shard_name == "" {
			//fmt.Println("等待")
			time.Sleep(time.Second * 1)
			continue
		} else {
			break
		}
	}
	fmt.Println("出来了！！shard=", Shard)
	Connect_success = true
	if Connect_success == true {
		fmt.Println("Connect_success=true")
	}
	for i := 0; i < 20; i++ {
		time.Sleep(time.Second) //等待10s
		if Judge_leader == true {
			break
		} else {
			time.Sleep(time.Second)
			if i == 15 {
				fmt.Println("该节点不是leader，因此不用建立连接")
				return
			}
		}
	}
	fmt.Println("该点是leader因此建立连接")
	Flag_init()
	l = newline()
	go begin()
}
func Send_flag1() {

	for {
		if Shard == 0 {
			time.Sleep(time.Second)
		} else {
			break
		}
	}
	Send_flag = make([]bool, Shard)
	for i := 0; i < Shard; i++ {
		Send_flag[i] = false
	}
	fmt.Println("初始化完成flag")
	go time1()
}
func init() {

	Shard_init()

	go figure_Shard()

	go Send_flag1()

}
func receiveloop(conn *websocket.Conn, shard string, i int) {
	for {
		_, _, err := conn.ReadMessage()
		//fmt.Println(string(p))
		if err != nil {
			fmt.Println("连接中断")

			fmt.Println(err)
			ip := l.target[shard][i]
			fmt.Println("无法连接", ip)
			c, _, err := l.connect(ip)
			if err != nil {
				fmt.Println("连接失败")
				fmt.Println(err)
				go receiveloop(c, shard, i)
				return
			}
			go receiveloop(c, shard, i)
			return
		}
		//fmt.Println(string(p))
	}
}

func Find_conns(flag string) int {
	for {
		rand.Seed(time.Now().UnixNano())
		rnd := rand.Intn(9)
		//fmt.Println("rnd=",rnd)
		if Flag_conn[flag][rnd] == false {
			return rnd
		}

		//fmt.Println("等待释放",string(flag+65),"资源")
		time.Sleep(time.Millisecond * 100)
	}

	return 0
}

func UseConnect(key string, ip string) (*websocket.Conn, int) {
	for {
		if Connect_success == false {
			fmt.Println("还未初始化完成请稍等～")
			time.Sleep(time.Second)
		} else if Connect_success == true {
			break
		}
	}

	if ip == "localhost" {
		Flag_conn["Localhost"][0] = true
		c := l.conns["Localhost"][0]
		fmt.Println("取到本地连接")
		return c, 0
	}

	rnd := Find_conns(key)
	Flag_conn[key][rnd] = true
	c := l.conns[key][rnd]
	return c, rnd
}

//连接函数
func (l *Line) connect(host string) (*websocket.Conn, *http.Response, error) {
	u := url.URL{Scheme: "ws", Host: host, Path: "/websocket"}
	return websocket.DefaultDialer.Dial(u.String(), nil)
}
func Connect(host string) (*websocket.Conn, *http.Response, error) {
	u := url.URL{Scheme: "ws", Host: host, Path: "/websocket"}
	return websocket.DefaultDialer.Dial(u.String(), nil)
}

//产生新的连接类型
func NewLine(target map[string][]string) *Line {
	//sum是算整体网络的节点个数，为了开辟相当的空间
	var sum int
	sum = 0
	for shard := range target {
		sum += len(target[shard])
	}
	return &Line{
		target: target,                                  //目标节点地址
		conns:  make(map[string][]*websocket.Conn, sum), //连接地址
	}
}

func (l *Line) ReStart(ip string, shard string, i int) {
	fmt.Println("连接出错,等待2s自动重连", ip)
	time.Sleep(time.Second * 2)
	c, _, err := Connect(ip)
	if err != nil {
		fmt.Println(err)
		go l.ReStart(ip, shard, i)
		return
	}
	l.conns[shard][i] = c
	go receiveloop(c, shard, i)
	return

}
func ReStart1(shard string, i int) *websocket.Conn {
	ip := l.target[shard][i]
	fmt.Println("连接出错,等待2s自动重连", ip)
	c, _, err := Connect(ip)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	l.conns[shard][i] = c
	go receiveloop(c, shard, i)
	return c

}
func (l *Line) Start() error {
	//time.Sleep(time.Second * 20)
	for shard := range l.target {
		l.conns[shard] = make([]*websocket.Conn, len(l.target[shard]))

		for i, ip := range l.target[shard] {
			//fmt.Println("连接",ip)
			c, _, err := l.connect(ip)
			if err != nil {
				go l.ReStart(ip, shard, i)
				continue
			}
			l.conns[shard][i] = c
			//go receiveloop(c, shard, i)
		}

	}
	fmt.Println("连接完成！")

	go l.KeepAlive()

	fmt.Println("完成～～～")
	return nil
}

//发送消息，随机取一个连接给目标节点发送信息
func (l *Line) SendMessageTrans(message json.RawMessage, Receiver string, Sender string) error {
	rc := &rpctypes.RPCRequest{
		JSONRPC:  "2.0",
		Sender:   Sender,
		Receiver: Receiver,
		Flag:     0,
		ID:       rpctypes.JSONRPCStringID("trans"),
		Method:   "broadcast_tx_commit",
		Params:   message,
	}
	rand.Seed(time.Now().Unix())
	//rnd := rand.Intn(4)
	c := l.conns[Receiver][0]
	err := c.WriteJSON(rc)
	if err != nil {
		return err
	}
	return nil
}
func (l *Line) SendMessageCommit(message json.RawMessage, Receiver string, Sender string) error {
	rc := &rpctypes.RPCRequest{
		JSONRPC:  "2.0",
		Sender:   Sender,
		Receiver: Receiver,
		Flag:     0,
		ID:       rpctypes.JSONRPCStringID("commit"),
		Method:   "broadcast_tx_commit",
		Params:   message,
	}
	rand.Seed(time.Now().Unix())
	rnd := rand.Intn(4)
	c := l.conns[Receiver][rnd]
	err := c.WriteJSON(rc)
	if err != nil {
		return err
	}
	return nil
}
func (l *Line) ReceiveMessage(key string, connindex int) error {
	c := l.conns[key][connindex]
	for {
		//第二个下划线指的是返回的信息，在下一步进行使用
		_, _, err := c.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				return err
			}
			return nil
		}

		//if t.stopped || t.connsBroken[connIndex] {
		//	return
		//}
	}
}
