package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	crand "crypto/rand"
	"flag"
	"fmt"

	//"github.com/gorilla/websocket"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/kit/log/term"

	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
	tmrpc "github.com/tendermint/tendermint/rpc/client"
)

var logger = log.NewNopLogger()

type plist []*ecdsa.PrivateKey

func main() {

	//durationInt表示持续时间
	//txsRate 发送交易个数
	//connections表示链接的个数
	//txSize 表示tx的大小
	//verbose 冗余
	//outputFormat 输出格式
	//shard 分片
	//broadcastTxmethod 传播方法
	//allshard 全分片
	//relayrate 跨片比例
	var durationInt, txsRate, connections, txSize, relayrate int
	var verbose bool
	var outputFormat, broadcastTxMethod, allshard string
	var shard string
	flagSet := flag.NewFlagSet("tm-bench", flag.ExitOnError)
	//初始化数值
	flagSet.IntVar(&connections, "c", 1, "Connections to keep open per endpoint")
	flagSet.IntVar(&durationInt, "T", 10, "Exit after the specified amount of time in seconds")
	flagSet.IntVar(&txsRate, "r", 1000, "Txs per second to send in a connection")
	flagSet.IntVar(&txSize, "s", 250, "The size of a transaction in bytes, must be greater than or equal to 40.")
	flagSet.StringVar(&outputFormat, "output-format", "plain", "Output format: plain or json")
	flagSet.StringVar(&shard, "shard", "0", "now shard of tendermint")
	flagSet.IntVar(&relayrate, "rate", 2, "relay rate")

	flagSet.StringVar(&allshard, "as", "0,1,2,3", "shard of tendermint")
	flagSet.StringVar(&broadcastTxMethod, "broadcast-tx-method", "async", "Broadcast method: async (no guarantees; fastest), sync (ensures tx is checked) or commit (ensures tx is checked and committed; slowest)")
	flagSet.BoolVar(&verbose, "v", false, "Verbose output")
	flagSet.Usage = func() {
		fmt.Println(`Tendermint blockchain benchmarking tool.

Usage:
	tm-bench [-c 1] [-T 10] [-r 1000] [-s 250] [endpoints] [-output-format <plain|json> [-broadcast-tx-method <async|sync|commit>]]

Examples:
	tm-bench localhost:26657`)
		fmt.Println("Flags:")
		flagSet.PrintDefaults()
	}
	//
	flagSet.Parse(os.Args[1:])

	if flagSet.NArg() == 0 {
		flagSet.Usage()
		os.Exit(1)
	}
	//fmt.Println("我使用了连接！！！")
	//c1 := myline.UseConnect("A",0)
	//go func(c *websocket.Conn){
	//	_,p,_:=c.ReadMessage()
	//	fmt.Println(p)
	//}(c1)

	//fmt.Println("连接关闭！！！")
	if verbose {
		if outputFormat == "json" {
			printErrorAndExit("Verbose mode not supported with json output.")
		}
		// Color errors red
		colorFn := func(keyvals ...interface{}) term.FgBgColor {
			for i := 1; i < len(keyvals); i += 2 {
				if _, ok := keyvals[i].(error); ok {
					return term.FgBgColor{Fg: term.White, Bg: term.Red}
				}
			}
			return term.FgBgColor{}
		}
		logger = log.NewTMLoggerWithColorFn(log.NewSyncWriter(os.Stdout), colorFn)

		fmt.Printf("Running %ds test @ %s\n", durationInt, flagSet.Arg(0))
	}

	if txSize < 40 {
		printErrorAndExit("The size of a transaction must be greater than or equal to 40.")
	}

	if broadcastTxMethod != "async" &&
		broadcastTxMethod != "sync" &&
		broadcastTxMethod != "commit" {
		printErrorAndExit("broadcast-tx-method should be either 'sync', 'async' or 'commit'.")
	}

	var (
		endpoints     = strings.Split(flagSet.Arg(0), ",")
		client        = tmrpc.NewHTTP(endpoints[0], "/websocket")
		initialHeight = latestBlockHeight(client)
	)

	logger.Info("Latest block height", "h", initialHeight)
	//开始创建client,发送交易
	var transacters []*transacter
	allSahrd := strings.Split(allshard, ",")
	transacters = startTransacters(
		endpoints,
		connections,
		txsRate,
		txSize,
		shard,
		allSahrd,
		relayrate,
		"broadcast_tx_"+broadcastTxMethod)

	// Stop upon receiving SIGTERM or CTRL-C.
	//创建各个分片的账户并写入文件中

	cmn.TrapSignal(logger, func() {
		for _, t := range transacters {
			t.Stop()
		}
	})

	// Wait until transacters have begun until we get the start time.
	timeStart := time.Now()
	//fmt.Println(timeStart)
	logger.Info("Time last transacter started", "t", timeStart)

	duration := time.Duration(durationInt) * time.Second

	timeEnd := timeStart.Add(duration)
	logger.Info("End time for calculation", "t", timeEnd)

	<-time.After(duration)
	for i, t := range transacters {
		t.Stop()
		numCrashes := countCrashes(t.connsBroken)
		if numCrashes != 0 {
			fmt.Printf("%d connections crashed on transacter #%d\n", numCrashes, i)
		}
	}
	logger.Debug("Time all transacters stopped", "t", time.Now())

	for i, _ := range endpoints {
		client = tmrpc.NewHTTP(endpoints[i], "/websocket")
		stats, err := calculateStatistics(
			client,
			initialHeight,
			timeStart,
			durationInt,
			int64(txsRate),
		)
		if err != nil {
			printErrorAndExit(err.Error())
		}
		printStatistics(stats, outputFormat)
	}

}

func latestBlockHeight(client tmrpc.Client) int64 {
	status, err := client.Status()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return status.SyncInfo.LatestBlockHeight
}

func countCrashes(crashes []bool) int {
	count := 0
	for i := 0; i < len(crashes); i++ {
		if crashes[i] {
			count++
		}
	}
	return count
}

func createCount(allshard []string, num int) []plist {

	var count []plist
	for i := 0; i < len(allshard); i++ {
		var pl plist
		for j := 0; j < num; j++ {
			priv, _ := ecdsa.GenerateKey(elliptic.P256(), crand.Reader)
			pl = append(pl, priv)
		}
		count = append(count, pl)
	}
	return count
}

func startTransacters(
	endpoints []string,
	connections,
	txsRate int,
	txSize int,
	shard string,
	allshard []string,
	relayrate int,
	broadcastTxMethod string) []*transacter {
	transacters := make([]*transacter, len(endpoints))

	count := createCount(allshard, txsRate)
	iwg := sync.WaitGroup{}
	iwg.Add(len(endpoints))
	//先创建账户
	for i, e := range endpoints {
		flag := 0
		t := newTransacter(e, connections, txsRate, txSize, shard, allshard, relayrate, count, flag, broadcastTxMethod)
		t.SetLogger(logger)
		go func(i int) {
			defer iwg.Done()
			if err := t.Start(); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			transacters[i] = t
		}(i)
	}
	iwg.Wait()
	// fmt.Println("创建账户完成")
	//发送交易
	// wg := sync.WaitGroup{}
	// wg.Add(len(endpoints))
	// for i, e := range endpoints {
	// 	shard := allshard[i]
	// 	flag := 1
	// 	t := newTransacter(e, connections, txsRate, txSize, shard, allshard, relayrate, count, flag, broadcastTxMethod)
	// 	t.SetLogger(logger)
	// 	go func(i int) {
	// 		defer wg.Done()
	// 		if err := t.Start(); err != nil {
	// 			fmt.Fprintln(os.Stderr, err)
	// 			os.Exit(1)
	// 		}
	// 		transacters[i] = t
	// 	}(i)
	// }
	// wg.Wait()
	// fmt.Println("交易发送完成")
	return transacters
}

func printErrorAndExit(err string) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
