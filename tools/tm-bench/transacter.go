package main

import (
	"crypto/sha256"
	//	"encoding/binary"
	//	"encoding/hex"
	"crypto/ecdsa"
	"crypto/md5"
	crand "crypto/rand"
	"encoding/asn1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"

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

	tp "github.com/tendermint/tendermint/identypes"
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
	count             []plist
	flag              int

	conns       []*websocket.Conn
	connsBroken []bool
	startingWg  sync.WaitGroup
	endingWg    sync.WaitGroup
	stopped     bool

	logger log.Logger
}

func newTransacter(target string, connections, rate int, size int, shard string, allshard []string, relayrate int, count []plist, flag int, broadcastTxMethod string) *transacter {
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
		go t.sendLoop(i, t.flag)
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
func (t *transacter) sendLoop(connIndex int, index int) {
	initfinish :=false
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

	var txNumber = 1

	pingsTicker := time.NewTicker(pingPeriod)
	txsTicker := time.NewTicker(1 * time.Second)
	defer func() {
		pingsTicker.Stop()
		txsTicker.Stop()
		t.endingWg.Done()
	}()
	send_shard := deleteSlice(t.allshard, t.shard)

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
				var ntx []byte
				if initfinish == false {
					// if i >= 100 {
					// 	break //第一秒钟，有几笔交易就产生多少账户
					// }
					ntx = t.generateTx(t.shard, i)

				} else {
					ntx = t.updateTx(txNumber, send_shard, t.shard, t.relayrate, t.Rate)
				}
				paramsJSON, err := json.Marshal(map[string]interface{}{"tx": ntx})
				fmt.Println(string(ntx))
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
			initfinish =true
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

func bigint2str(r, s big.Int) string {
	coor := ecdsaSignature{X: &r, Y: &s}
	b, _ := asn1.Marshal(coor)
	return hex.EncodeToString(b)
}
func digest(Content string) []byte {
	origin := []byte(Content)

	// 生成md5 hash值
	digest_md5 := md5.New()
	digest_md5.Write(origin)

	return digest_md5.Sum(nil)
}

type ecdsaSignature struct {
	X, Y *big.Int
}

func pub2string(pub ecdsa.PublicKey) string {

	coor := ecdsaSignature{X: pub.X, Y: pub.Y}
	b, _ := asn1.Marshal(coor)

	return hex.EncodeToString(b)
}

func (t *transacter) createinitTxContent(shard string, i int) (string, string) {
	toint, _ := strconv.ParseInt(shard, 32, 64)
	index := toint
	//获取分片
	shardcount := t.count[index]
	//对所有账户存钱
	priv := shardcount[i]

	pub_s := priv.PublicKey
	tx_content := "_" + pub2string(pub_s) + "_1000000" + "_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	sig := "sig"
	return tx_content, sig

}

func (t *transacter) createRelayTxContent(shard_s string, shard_r string, rate int) (string, string) {
	//rate指的是该分片账户数量
	source := rand.NewSource(time.Now().Unix())
	newrand := rand.New(source)
	//产生转化交易金额
	num := newrand.Intn(100)
	toint, _ := strconv.ParseInt(shard_s, 32, 64)
	sendshard := t.count[toint]
	//拿取支付方账户信息
	priv := sendshard[newrand.Intn(rate)]
	pub_s := priv.PublicKey

	toint2, _ := strconv.ParseInt(shard_r, 32, 64)
	receiveshard := t.count[toint2]
	//拿取转入方账户信息
	priv_r := receiveshard[newrand.Intn(rate)]
	pub_r := priv_r.PublicKey
	//最后加入时间戳信息
	tx_content := pub2string(pub_s) + "_" + pub2string(pub_r) + "_" + strconv.Itoa(num) + "_" + strconv.FormatInt(time.Now().UnixNano(), 10) //添加时间戳来唯一标识该交易

	tr, ts, _ := ecdsa.Sign(crand.Reader, priv, digest(tx_content))
	sig := bigint2str(*tr, *ts)
	return tx_content, sig

}

func (t *transacter) createLocalTxContent(shard string, rate int) (string, string) {
	//rate指的是该分片账户数量
	source := rand.NewSource(time.Now().Unix())
	newrand := rand.New(source)
	//金额可以随便
	num := newrand.Intn(100)

	toint, _ := strconv.ParseInt(shard, 32, 64)

	shardcount := t.count[toint]
	priv := shardcount[newrand.Intn(rate)]
	pub_s := priv.PublicKey

	priv_r := shardcount[newrand.Intn(rate)]
	pub_r := priv_r.PublicKey

	tx_content := pub2string(pub_s) + "_" + pub2string(pub_r) + "_" + strconv.Itoa(num) + "_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	tr, ts, _ := ecdsa.Sign(crand.Reader, priv, digest(tx_content))
	sig := bigint2str(*tr, *ts)
	return tx_content, sig

}

//TX考虑当前分片中实际账户
func (t *transacter) generateTx(shard string, index int) []byte {
	content, sig := t.createinitTxContent(shard, index)
	tx := &tp.TX{
		Txtype:      "init",
		Sender:      shard,
		Receiver:    "",
		ID:          sha256.Sum256([]byte(content)),
		Content:     content,
		TxSignature: sig,
		Operate:     0}
	res, _ := json.Marshal(tx)

	return res
}

// warning, mutates input byte slice
func (t *transacter) updateTx(txNumber int, send_shard []string, shard string, rate int, txnum int) []byte {

	var res []byte
	if txNumber%rate == 0 {
		step := len(send_shard)
		content, sig := t.createRelayTxContent(shard, send_shard[txNumber%step], txnum)
		tx := &tp.TX{
			Txtype:      "relaytx",
			Sender:      shard,
			Receiver:    send_shard[txNumber%step],
			ID:          sha256.Sum256([]byte(content)),
			Content:     content,
			TxSignature: sig,
			Operate:     0}
		res, _ = json.Marshal(tx)
	} else {
		content, sig := t.createLocalTxContent(shard, txnum)
		tx := &tp.TX{
			Txtype:      "tx",
			Sender:      "",
			Receiver:    "",
			ID:          sha256.Sum256([]byte(content)),
			Content:     content,
			TxSignature: sig}
		res, _ = json.Marshal(tx)
	}
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
