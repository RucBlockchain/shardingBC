package main

import (
	"crypto/sha256"
	//	"encoding/binary"
	//	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

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

type TX struct {
	Txtype   string
	Sender   string
	Receiver string
	ID       [sha256.Size]byte
	Content  []string
}

type transacter struct {
	Target            string
	Rate              int
	Size              int
	Connections       int
	BroadcastTxMethod string
	shard             string
	allshard          string

	conns       []*websocket.Conn
	connsBroken []bool
	startingWg  sync.WaitGroup
	endingWg    sync.WaitGroup
	stopped     bool

	logger log.Logger
}

func newTransacter(target string, connections, rate int, size int, shard string, allshard string, broadcastTxMethod string) *transacter {
	return &transacter{
		Target:            target,
		Rate:              rate,
		Size:              size,
		Connections:       connections,
		BroadcastTxMethod: broadcastTxMethod,
		shard:             shard,
		allshard:          allshard,
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
	//fmt.Println("开始————————————————————————————————————————————————————————————")
	for i := 0; i < t.Connections; i++ {
		c, _, err := connect(t.Target)
		if err != nil {
			return err
		}
		t.conns[i] = c
	}

	t.startingWg.Add(t.Connections)
	t.endingWg.Add(2 * t.Connections)
	for i := 0; i < t.Connections; i++ {
		go t.sendLoop(i)
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
func (t *transacter) sendLoop(connIndex int) {
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

	var txNumber = 0

	pingsTicker := time.NewTicker(pingPeriod)
	txsTicker := time.NewTicker(1 * time.Second)
	defer func() {
		pingsTicker.Stop()
		txsTicker.Stop()
		t.endingWg.Done()
		//fmt.Println("关闭进程")
		//time.Sleep(time.Second * 100)
	}()

	// hash of the host name is a part of each tx
	//var hostnameHash [sha256.Size]byte
	//hostname, err := os.Hostname()
	//if err != nil {
	//	hostname = "127.0.0.1"
	//}
	//hostnameHash = sha256.Sum256([]byte(hostname))
	// each transaction embeds connection index, tx number and hash of the hostname
	// we update the tx number between successive txs

	//tx := generateTx(connIndex, txNumber, t.Size, hostnameHash)
	//tx := generateTx(connIndex, txNumber, t.Size, hostnameHash,t.shard)
	//txHex := make([]byte, len(tx)*2)
	//hex.Encode(txHex, tx)
	allShard := strings.Split(t.allshard, ",")
	send_shard := deleteSlice(allShard, t.shard)

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
			//rate是每秒发送消息的数量
			for i := 0; i < t.Rate; i++ {
				//update tx number of the tx, and the corresponding hex
				//updateTx(tx, txHex, txNumber,send_shard,t.shard)
				ntx := updateTx(txNumber, send_shard, t.shard)
				//fmt.Println(string(ntx))

				paramsJSON, err := json.Marshal(map[string]interface{}{"tx": ntx})

				if err != nil {
					fmt.Printf("failed to encode params: %v\n", err)
					os.Exit(1)
				}
				rawParamsJSON := json.RawMessage(paramsJSON)

				c.SetWriteDeadline(now.Add(sendTimeout))
				err = c.WriteJSON(rpctypes.RPCRequest{
					JSONRPC: "2.0",
					ID:      rpctypes.JSONRPCStringID("tm-bench"),
					Method:  t.BroadcastTxMethod,
					Params:  rawParamsJSON,
				})
				//c1.SetWriteDeadline(now.Add(sendTimeout))
				//fmt.Println("发送消息")
				//err1 := c.WriteJSON(rpctypes.RPCRequest{
				//	JSONRPC: "2.0",
				//	ID:      rpctypes.JSONRPCStringID("tm-bench"),
				//	Method:  t.BroadcastTxMethod,
				//	Params:  rawParamsJSON,
				//})
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

/*
func generateTx(connIndex int, txNumber int, txSize int, hostnameHash [sha256.Size]byte) []byte {
	t := time.Now()
	timestamp := strconv.FormatInt(t.UTC().UnixNano(), 10)
	headstr:="tx="+timestamp[:9]
	tx := make([]byte, txSize)
	binary.PutUvarint(tx[12:20], uint64(connIndex))
	binary.PutUvarint(tx[20:28], uint64(txNumber))
	copy(tx[28:40], hostnameHash[:12])
	//binary.PutUvarint(tx[32:40], uint64(time.Now().Unix()))
	temp:=[]byte(headstr)
	copy(tx[:12],temp)
	// 40-* random data

	return tx
}

// warning, mutates input byte slice
func updateTx(tx []byte, txHex []byte, txNumber int,send_shard []string,shard string) {

	binary.PutUvarint(tx[8:16], uint64(txNumber))
	hexUpdate := make([]byte, 16)
	hex.Encode(hexUpdate, tx[8:16])
	t := time.Now()
	timestamp := strconv.FormatInt(t.UTC().UnixNano(), 10)
	headstr:="tx="+timestamp[:9]
	for i := 16; i < 32; i++ {
		txHex[i] = hexUpdate[i-16]
	}
	if(txNumber%2==0){
		step:=len(send_shard)
		txHead:="relaytx,"+shard+","+send_shard[txNumber%step]+"="
		temp:=[]byte(txHead)
		copy(txHex[0:12],temp)
	}else{
		temp:=[]byte(headstr)
		copy(txHex[:12],temp)
	}
}


*/
func generateTx(connIndex int, txNumber int, txSize int, hostnameHash [sha256.Size]byte, shard string) []byte {

	t := time.Now()
	timestamp := strconv.FormatInt(t.UTC().UnixNano(), 10)
	content := shard + strconv.Itoa(txNumber) + timestamp
	tx := &TX{
		Txtype:   "tx",
		Sender:   "",
		Receiver: "",
		ID:       sha256.Sum256([]byte(content)),
		Content:  []string{content}}
	res, _ := json.Marshal(tx)
	restx := make([]byte, txSize)
	txLength := len(res)
	copy(restx[:txLength], res)

	return restx
}

// warning, mutates input byte slice
func updateTx(txNumber int, send_shard []string, shard string) []byte {

	t := time.Now()
	timestamp := strconv.FormatInt(t.UTC().UnixNano(), 10)
	content := shard + strconv.Itoa(txNumber) + timestamp
	var res []byte
	//if txNumber%2 == 0 {
	//	step := len(send_shard)
	//	tx := &TX{
	//		Txtype:   "relaytx",
	//		Sender:   shard,
	//		Receiver: send_shard[txNumber%step],
	//		ID:       sha256.Sum256([]byte(content)),
	//		Content:  []string{content}}
	//	res, _ = json.Marshal(tx)
	//} else {
		tx := &TX{
			Txtype:   "tx",
			Sender:   "",
			Receiver: "",
			ID:       sha256.Sum256([]byte(content)),
			Content:  []string{content}}
		res, _ = json.Marshal(tx)
	//}

	return res

}
func sendaddtx(tx []byte) []byte {
	var t TX
	json.Unmarshal(tx, &t)
	t.Txtype = "addtx"
	res, _ := json.Marshal(t)
	return res
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
