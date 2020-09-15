package main

import (
	//	"encoding/binary"
	//	"encoding/hex"
	"encoding/json"
	"fmt"


	// it is ok to use math/rand here: we do not need a cryptographically secure random
	// number generator here and we can run the tests a bit faster
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"


	"github.com/tendermint/tendermint/libs/log"
	rpctypes "github.com/tendermint/tendermint/rpc/lib/types"
)

const (
	sendTimeout = 10 * time.Second
	// see https://github.com/tendermint/tendermint/blob/master/rpc/lib/server/handlers.go
	pingPeriod = (30 * 9 / 10) * time.Second
)

type transacter struct {
	Target            string
	Rate              int
	Size              int
	Connections       int
	BroadcastTxMethod string
	shard             string
	allshard          []string
	relayrate         int
	count             [][]byte
	flag              int

	conns       []*websocket.Conn
	connsBroken []bool
	startingWg  sync.WaitGroup
	endingWg    sync.WaitGroup
	stopped     bool

	logger log.Logger
}

func newTransacter(target string, connections, rate int, size int, shard string, allshard []string, relayrate int, count [][]byte, flag int, broadcastTxMethod string) *transacter {
	return &transacter{
		Target:            target,
		Rate:              rate,
		Size:              size,
		Connections:       connections,
		BroadcastTxMethod: broadcastTxMethod,
		shard:             shard,
		allshard:          allshard,
		relayrate:         relayrate,
		count:             count,
		flag:              flag,
		conns:             make([]*websocket.Conn, connections),
		connsBroken:       make([]bool, connections),
		logger:            log.NewNopLogger(),
	}
}

// SetLogger lets you set your own logger
func (t *transacter) SetLogger(l log.Logger) {
	t.logger = l
}

// Start opens N = `t.Connections` connections to the target and creates read
// and write goroutines for each connection.
func (t *transacter) Start() error {
	t.stopped = false

	rand.Seed(time.Now().Unix())
	//连接到某个节点，有多条连接
	//考虑把t.Target换成一个数组或切片
	//连接的发送方变多
	for i := 0; i < t.Connections; i++ {
		c, _, err := connect(t.Target)
		if err != nil {
			return err
		}
		t.conns[i] = c
	}
	t.startingWg.Add(t.Connections)
	//发送init交易保证不会出现支付方还没被确认
	//这部分不需要了
	for i := 0; i < t.Connections; i++ {//多个连接并行执行sendloop
		go t.sendLoop(i, t.flag,false)
	}
	t.startingWg.Wait()
	time.Sleep(1*time.Second)
	t.startingWg.Add(t.Connections)
	t.endingWg.Add(2 * t.Connections)
	for i := 0; i < t.Connections; i++ {
		//这里的flag是干吗的
		go t.sendLoop(i, t.flag,true)
		go t.receiveLoop(i)
	}

	t.startingWg.Wait()
	return nil
}

// Stop closes the connections.
func (t *transacter) Stop() {
	t.stopped = true
	t.endingWg.Wait()
	for _, c := range t.conns {
		c.Close()
	}
}

// receiveLoop reads messages from the connection (empty in case of
// `broadcast_tx_async`).
func (t *transacter) receiveLoop(connIndex int) {
	c := t.conns[connIndex]
	defer t.endingWg.Done()
	for {
		_, _, err := c.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				t.logger.Error(
					fmt.Sprintf("failed to read response on conn %d", connIndex),
					"err",
					err,
				)
			}
			return
		}
		if t.stopped || t.connsBroken[connIndex] {
			return
		}
	}
}

// sendLoop generates transactions at a given rate.
// 为什么index这个参数在这个函数中没有使用
func (t *transacter) sendLoop(connIndex int, index int,init bool) {
	started := false
	// Close the starting waitgroup, in the event that this fails to start
	defer func() {

		if !started {
			t.startingWg.Done()
		}
	}()
	c := t.conns[connIndex]

	c.SetPingHandler(func(message string) error {
		err := c.WriteControl(websocket.PongMessage, []byte(message), time.Now().Add(sendTimeout))
		if err == websocket.ErrCloseSent {
			return nil
		} else if e, ok := err.(net.Error); ok && e.Temporary() {
			return nil
		}
		return err
	})
	logger := t.logger.With("addr", c.RemoteAddr())
	var txNumber = 0 //用来表示已经取到了
	leftNum := len(t.count) - len(t.count) / t.Rate * t.Rate //剩余交易总数

	pingsTicker := time.NewTicker(pingPeriod)
	txsTicker := time.NewTicker(1 * time.Second)
	defer func() {
		pingsTicker.Stop()
		txsTicker.Stop()
		t.endingWg.Done()
	}()
	//send_shard := deleteSlice(t.allshard, t.shard)//不需要了

	for {
		select {
		case <-txsTicker.C:
			startTime := time.Now()
			endTime := startTime.Add(time.Second)
			numTxSent := t.Rate
			if !started {
				t.startingWg.Done()
				started = true
			}

			now := time.Now()

			//得到循环次数，把txcount/T的余数放在
			var cir int
			if  leftNum > 0 {
				cir = t.Rate + 1
			}else {
				cir = t.Rate
			}
			leftNum--

			//rate是每秒发送消息的数量
			for i := 0; i < cir; i++ {//由于一段时间内某个分片的交易不一定能整除持续时间，需要对循环的退出条件进行修改
				var ntx []byte

				//ntx = t.updateTx(txNumber, send_shard, t.shard, t.relayrate, t.Rate)//生成一个交易t.count[i]
				ntx = t.count[txNumber]
				//生成这个交易后只有一处使用了这个交易，使用这个交易的地方是不是把该交易发送到一个发送方
				//updateTx需要修改，改为根据i的值选出属于这个发送方的一条交易
				//fmt.Println(string(ntx))
				paramsJSON, err := json.Marshal(map[string]interface{}{"tx": ntx})
				if err != nil {
					fmt.Printf("failed to encode params: %v\n", err)
					os.Exit(1)
				}
				rawParamsJSON := json.RawMessage(paramsJSON)//把交易转换成

				c.SetWriteDeadline(now.Add(sendTimeout))
				err = c.WriteJSON(rpctypes.RPCRequest{
					JSONRPC: "2.0",
					ID:      rpctypes.JSONRPCStringID("tm-bench"),
					Method:  t.BroadcastTxMethod,
					Params:  rawParamsJSON,
				})
				if err != nil {
					err = errors.Wrap(err,
						fmt.Sprintf("txs send failed on connection #%d", connIndex))
					t.connsBroken[connIndex] = true
					logger.Error(err.Error())
					return
				}
				// cache the time.Now() reads to save time.
				if i%5 == 0 {
					now = time.Now()
					if now.After(endTime) {
						// Plus one accounts for sending this tx
						numTxSent = i + 1
						break
					}
				}

				txNumber++
			}
			timeToSend := time.Since(startTime)
			logger.Info(fmt.Sprintf("sent %d transactions", numTxSent), "took", timeToSend)
			if timeToSend < 1*time.Second {
				sleepTime := time.Second - timeToSend
				logger.Debug(fmt.Sprintf("connection #%d is sleeping for %f seconds", connIndex, sleepTime.Seconds()))
				time.Sleep(sleepTime)
			}

		case <-pingsTicker.C:
			// go-rpc server closes the connection in the absence of pings
			c.SetWriteDeadline(time.Now().Add(sendTimeout))
			if err := c.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				err = errors.Wrap(err,
					fmt.Sprintf("failed to write ping message on conn #%d", connIndex))
				logger.Error(err.Error())
				t.connsBroken[connIndex] = true
			}
		}
		if t.stopped {
			// To cleanly close a connection, a client should send a close
			// frame and wait for the server to close the connection.
			c.SetWriteDeadline(time.Now().Add(sendTimeout))
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				err = errors.Wrap(err,
					fmt.Sprintf("failed to write close message on conn #%d", connIndex))
				logger.Error(err.Error())
				t.connsBroken[connIndex] = true
			}

			return
		}
	}
}

func connect(host string) (*websocket.Conn, *http.Response, error) {
	u := url.URL{Scheme: "ws", Host: host, Path: "/websocket"}
	return websocket.DefaultDialer.Dial(u.String(), nil)
}

func deleteSlice(a []string, alp string) []string {
	ret := make([]string, 0, len(a))
	for _, val := range a {
		if val != alp {
			ret = append(ret, val)
		}
	}
	return ret
}
