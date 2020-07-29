package core

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"syscall"

	"github.com/pkg/errors"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/checkdb"
	myclient "github.com/tendermint/tendermint/client"
	tp "github.com/tendermint/tendermint/identypes"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	rpctypes "github.com/tendermint/tendermint/rpc/lib/types"
	"github.com/tendermint/tendermint/types"
)

//-----------------------------------------------------------------------------
// NOTE: tx should be signed, but this is only checked at the app level (not by Tendermint!)

// Returns right away, with no response. Does not wait for CheckTx nor
// DeliverTx results.
//
// Please refer to
// https://tendermint.com/docs/tendermint-core/using-tendermint.html#formatting
// for formatting/encoding rules.
//
//
// ```shell
// curl 'localhost:26657/broadcast_tx_async?tx="123"'
// ```
//
// ```go
// client := client.NewHTTP("tcp://0.0.0.0:26657", "/websocket")
// err := client.Start()
// if err != nil {
//   // handle error
// }
// defer client.Stop()
// result, err := client.BroadcastTxAsync("123")
// ```
//
// > The above command returns JSON structured like this:
//
// ```json
// {
// 	"error": "",
// 	"result": {
// 		"hash": "E39AAB7A537ABAA237831742DCE1117F187C3C52",
// 		"log": "",
// 		"data": "",
// 		"code": "0"
// 	},
// 	"id": "",
// 	"jsonrpc": "2.0"
// }
// ```
//
// ### Query Parameters
//
// | Parameter | Type | Default | Required | Description     |
// |-----------+------+---------+----------+-----------------|
// | tx        | Tx   | nil     | true     | The transaction |
func getShard() string {
	v, _ := syscall.Getenv("TASKID")
	return v
}
func Sum(bz []byte) []byte {
	h := sha256.Sum256(bz)
	return h[:]
}
func CmID(cm *tp.CrossMessages) string {
	heightHash := Sum([]byte(strconv.FormatInt(cm.Height, 10)))
	ID := append(heightHash, cm.CrossMerkleRoot...)
	return fmt.Sprintf("%X", ID)
}

func CheckDB(tx types.Tx) error {
	if cm := ParseData(tx); cm != nil {
		if cm.SrcZone==getShard(){
			//收到状态数据库的回执f
			//fmt.Println("收到回执并且执行删除",cm.Packages)
			mempool.ModifyCrossMessagelist(cm)
			return errors.New("更改crossMessages")
		}
		//fmt.Println("收到", cm.Height, "root", cm.CrossMerkleRoot,"SrcZone",cm.SrcZone,"DesZone",cm.DesZone)
		//relaynum := 0
		//addnum := 0
		//for i := 0; i < len(cm.Txlist); i++ {
		//	tx, _ := tp.NewTX(cm.Txlist[i])
		//	fmt.Println(tx)
		//	if tx.Txtype == "relaytx" {
		//		relaynum += 1
		//	} else if tx.Txtype == "addtx" {
		//		addnum += 1
		//	}
		//}
		//fmt.Println("relay交易数量", relaynum)
		//fmt.Println("addtx交易数量", addnum)
		cmid := CmID(cm)
		//fmt.Println("查询id", []byte(cmid))
		dbtx := checkdb.Search([]byte(cmid))
		if dbtx != nil {
			name := dbtx.SrcZone + "S"+cm.SrcIndex+":26657"
		//	fmt.Println("发送",name)
		//	fmt.Println("回执crossmessage"," 对方的height",dbtx.Height," cmroot",
		//	dbtx.CrossMerkleRoot,"SrcZone",dbtx.SrcZone,"DesZone",dbtx.DesZone,
		//)
			tx_package := []*tp.CrossMessages{}
			tx_package = append(tx_package, dbtx)
			for i := 0; i < len(tx_package); i++ {
				client := *myclient.NewHTTP(name, "/websocket")

				go client.BroadcastCrossMessageAsync(tx_package)
			}
			//fmt.Println("状态数据库返回")
			return errors.New("状态数据库返回")
		} else {
			return nil
		}

	}

	return nil
}
func ParseData(data types.Tx) (*tp.CrossMessages) {
	cm := new(tp.CrossMessages)
	err := json.Unmarshal(data, &cm)

	if err != nil {
		fmt.Println("ParseData Wrong")
	}
	if cm.Height==0 && cm.Txlist==nil{
		return nil
	} else {
		return cm
	}
}

func BroadcastTxAsync(ctx *rpctypes.Context, tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	//异步解决
	//if cm:=ParseData(tx);cm!=nil{//处理CrossMessage流程
	////TODO:需要完善此接口函数
	//	mempool.CheckCrossMessage(tx)//check这个消息如果通过则放入mempool之中
	//}else{//处理Tx流程
	//状态数据库先不检验
	//if Checkdbtest(tx) {
	//	return nil, errors.New("状态数据库直接返回")
	//}
	//err := CheckDB(tx)
	//if err != nil {
	//	return nil, err
	//}
	err := mempool.CheckTx(tx, nil)
	if err != nil {
		if err == errors.New("不合法交易") {
			if cm := ParseData(tx); cm != nil {
				//fmt.Println("交易cm不合法", cm)
			} else {
				//tx1, _ := tp.NewTX(tx)

				//fmt.Println("交易tx不合法", tx1)
			}
		}
		//fmt.Println("checktx的结果", err)
		return nil, err
	}
	//}

	return &ctypes.ResultBroadcastTx{Hash: tx.Hash()}, nil
}

// Returns with the response from CheckTx. Does not wait for DeliverTx result.
//
// Please refer to
// https://tendermint.com/docs/tendermint-core/using-tendermint.html#formatting
// for formatting/encoding rules.
//
// ```shell
// curl 'localhost:26657/broadcast_tx_sync?tx="456"'
// ```
//
// ```go
// client := client.NewHTTP("tcp://0.0.0.0:26657", "/websocket")
// err := client.Start()
// if err != nil {
//   // handle error
// }
// defer client.Stop()
// result, err := client.BroadcastTxSync("456")
// ```
//
// > The above command returns JSON structured like this:
//
// ```json
// {
// 	"jsonrpc": "2.0",
// 	"id": "",
// 	"result": {
// 		"code": "0",
// 		"data": "",
// 		"log": "",
// 		"hash": "0D33F2F03A5234F38706E43004489E061AC40A2E"
// 	},
// 	"error": ""
// }
// ```
//
// ### Query Parameters
//
// | Parameter | Type | Default | Required | Description     |
// |-----------+------+---------+----------+-----------------|
// | tx        | Tx   | nil     | true     | The transaction |
func BroadcastTxSync(ctx *rpctypes.Context, tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	resCh := make(chan *abci.Response, 1)
	err := mempool.CheckTx(tx, func(res *abci.Response) {
		resCh <- res
	})
	if err != nil {
		return nil, err
	}
	res := <-resCh
	r := res.GetCheckTx()
	return &ctypes.ResultBroadcastTx{
		Code: r.Code,
		Data: r.Data,
		Log:  r.Log,
		Hash: tx.Hash(),
	}, nil
}

// Returns with the responses from CheckTx and DeliverTx.
//
// IMPORTANT: use only for testing and development. In production, use
// BroadcastTxSync or BroadcastTxAsync. You can subscribe for the transaction
// result using JSONRPC via a websocket. See
// https://tendermint.com/docs/app-dev/subscribing-to-events-via-websocket.html
//
// CONTRACT: only returns error if mempool.CheckTx() errs or if we timeout
// waiting for tx to commit.
//
// If CheckTx or DeliverTx fail, no error will be returned, but the returned result
// will contain a non-OK ABCI code.
//
// Please refer to
// https://tendermint.com/docs/tendermint-core/using-tendermint.html#formatting
// for formatting/encoding rules.
//
//
// ```shell
// curl 'localhost:26657/broadcast_tx_commit?tx="789"'
// ```
//
// ```go
// client := client.NewHTTP("tcp://0.0.0.0:26657", "/websocket")
// err := client.Start()
// if err != nil {
//   // handle error
// }
// defer client.Stop()
// result, err := client.BroadcastTxCommit("789")
// ```
//
// > The above command returns JSON structured like this:
//
// ```json
// {
// 	"error": "",
// 	"result": {
// 		"height": "26682",
// 		"hash": "75CA0F856A4DA078FC4911580360E70CEFB2EBEE",
// 		"deliver_tx": {
// 			"log": "",
// 			"data": "",
// 			"code": "0"
// 		},
// 		"check_tx": {
// 			"log": "",
// 			"data": "",
// 			"code": "0"
// 		}
// 	},
// 	"id": "",
// 	"jsonrpc": "2.0"
// }
// ```
//
// ### Query Parameters
//
// | Parameter | Type | Default | Required | Description     |
// |-----------+------+---------+----------+-----------------|
// | tx        | Tx   | nil     | true     | The transaction |
func BroadcastTxCommit(ctx *rpctypes.Context, tx types.Tx) (*ctypes.ResultBroadcastTxCommit, error) {

	subscriber := ctx.RemoteAddr()
	if eventBus.NumClients() >= config.MaxSubscriptionClients {
		return nil, fmt.Errorf("max_subscription_clients %d reached", config.MaxSubscriptionClients)
	} else if eventBus.NumClientSubscriptions(subscriber) >= config.MaxSubscriptionsPerClient {
		return nil, fmt.Errorf("max_subscriptions_per_client %d reached", config.MaxSubscriptionsPerClient)
	}

	// Subscribe to tx being committed in block.
	subCtx, cancel := context.WithTimeout(ctx.Context(), SubscribeTimeout)
	defer cancel()
	q := types.EventQueryTxFor(tx)
	deliverTxSub, err := eventBus.Subscribe(subCtx, subscriber, q)
	if err != nil {
		err = errors.Wrap(err, "failed to subscribe to tx")
		logger.Error("Error on broadcast_tx_commit", "err", err)
		return nil, err
	}
	defer eventBus.Unsubscribe(context.Background(), subscriber, q)

	// Broadcast tx and wait for CheckTx result
	checkTxResCh := make(chan *abci.Response, 1)
	err = mempool.CheckTx(tx, func(res *abci.Response) {
		checkTxResCh <- res
	})
	if err != nil {
		logger.Error("Error on broadcastTxCommit", "err", err)
		return nil, fmt.Errorf("Error on broadcastTxCommit: %v", err)
	}
	checkTxResMsg := <-checkTxResCh
	checkTxRes := checkTxResMsg.GetCheckTx()
	if checkTxRes.Code != abci.CodeTypeOK {
		return &ctypes.ResultBroadcastTxCommit{
			CheckTx:   *checkTxRes,
			DeliverTx: abci.ResponseDeliverTx{},
			Hash:      tx.Hash(),
		}, nil
	}

	// Wait for the tx to be included in a block or timeout.
	select {
	case msg := <-deliverTxSub.Out(): // The tx was included in a block.
		deliverTxRes := msg.Data().(types.EventDataTx)
		return &ctypes.ResultBroadcastTxCommit{
			CheckTx:   *checkTxRes,
			DeliverTx: deliverTxRes.Result,
			Hash:      tx.Hash(),
			Height:    deliverTxRes.Height,
		}, nil
	case <-deliverTxSub.Cancelled():
		var reason string
		if deliverTxSub.Err() == nil {
			reason = "Tendermint exited"
		} else {
			reason = deliverTxSub.Err().Error()
		}
		err = fmt.Errorf("deliverTxSub was cancelled (reason: %s)", reason)
		logger.Error("Error on broadcastTxCommit", "err", err)
		return &ctypes.ResultBroadcastTxCommit{
			CheckTx:   *checkTxRes,
			DeliverTx: abci.ResponseDeliverTx{},
			Hash:      tx.Hash(),
		}, err
	case <-time.After(config.TimeoutBroadcastTxCommit):
		err = errors.New("Timed out waiting for tx to be included in a block")
		logger.Error("Error on broadcastTxCommit", "err", err)
		return &ctypes.ResultBroadcastTxCommit{
			CheckTx:   *checkTxRes,
			DeliverTx: abci.ResponseDeliverTx{},
			Hash:      tx.Hash(),
		}, err
	}
}

// Get unconfirmed transactions (maximum ?limit entries) including their number.
//
// ```shell
// curl 'localhost:26657/unconfirmed_txs'
// ```
//
// ```go
// client := client.NewHTTP("tcp://0.0.0.0:26657", "/websocket")
// err := client.Start()
// if err != nil {
//   // handle error
// }
// defer client.Stop()
// result, err := client.UnconfirmedTxs()
// ```
//
// > The above command returns JSON structured like this:
//
// ```json
// {
//   "result" : {
//       "txs" : [],
//       "total_bytes" : "0",
//       "n_txs" : "0",
//       "total" : "0"
//     },
//     "jsonrpc" : "2.0",
//     "id" : ""
//   }
// ```
//
// ### Query Parameters
//
// | Parameter | Type | Default | Required | Description                          |
// |-----------+------+---------+----------+--------------------------------------|
// | limit     | int  | 30      | false    | Maximum number of entries (max: 100) |
// ```
func UnconfirmedTxs(ctx *rpctypes.Context, limit int) (*ctypes.ResultUnconfirmedTxs, error) {
	// reuse per_page validator
	limit = validatePerPage(limit)

	txs := mempool.ReapMaxTxs(limit)
	return &ctypes.ResultUnconfirmedTxs{
		Count:      len(txs),
		Total:      mempool.Size(),
		TotalBytes: mempool.TxsBytes(),
		Txs:        txs}, nil
}

// Get number of unconfirmed transactions.
//
// ```shell
// curl 'localhost:26657/num_unconfirmed_txs'
// ```
//
// ```go
// client := client.NewHTTP("tcp://0.0.0.0:26657", "/websocket")
// err := client.Start()
// if err != nil {
//   // handle error
// }
// defer client.Stop()
// result, err := client.UnconfirmedTxs()
// ```
//
// > The above command returns JSON structured like this:
//
// ```json
// {
//   "jsonrpc" : "2.0",
//   "id" : "",
//   "result" : {
//     "n_txs" : "0",
//     "total_bytes" : "0",
//     "txs" : null,
//     "total" : "0"
//   }
// }
// ```
func NumUnconfirmedTxs(ctx *rpctypes.Context) (*ctypes.ResultUnconfirmedTxs, error) {
	return &ctypes.ResultUnconfirmedTxs{
		Count:      mempool.Size(),
		Total:      mempool.Size(),
		TotalBytes: mempool.TxsBytes()}, nil
}
