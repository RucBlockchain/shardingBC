package main

import (
	"crypto/ecdsa"
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
	var outputFormat, broadcastTxMethod, allshard, datafile string
	//var shard string
	flagSet := flag.NewFlagSet("tm-bench", flag.ExitOnError)
	//初始化数值
	flagSet.IntVar(&connections, "c", 1, "Connections to keep open per endpoint")
	flagSet.IntVar(&durationInt, "T", 10, "Exit after the specified amount of time in seconds")
	flagSet.IntVar(&txsRate, "r", 1000, "Txs per second to send in a connection")
	flagSet.IntVar(&txSize, "s", 250, "The size of a transaction in bytes, must be greater than or equal to 40.")
	flagSet.StringVar(&outputFormat, "output-format", "plain", "Output format: plain or json")
	flagSet.StringVar(&datafile, "datafile", "token_transfers.csv", "The name of data file")
	//flagSet.StringVar(&shard, "shard", "0", "now shard of tendermint")
	flagSet.IntVar(&relayrate, "rate", 2, "relay rate")

	flagSet.StringVar(&allshard, "as", "0", "shard of tendermint")
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
		clients []*tmrpc.HTTP
		initHeights []int64
	)
	//创建一组分片leader小组链接
	clients = make([]*tmrpc.HTTP,len(endpoints))
	initHeights = make([]int64, len(endpoints))
	for i:=0;i<len(endpoints);i++{
		clients[i] = tmrpc.NewHTTP(endpoints[i], "/websocket")
		initHeights[i] = latestBlockHeight(clients[i])
		// fmt.Println("获取区块高度",initHeights[i])
	}
	//得到各分片链的初始化高度
	//logger.Info("Latest block height", "h", initialHeight)
	//开始创建client,发送交易
	var transacters []*transacter
	var rawdata map[string] Stas
	allSahrd := strings.Split(allshard, ",")
	transacters,rawdata = startTransacters(//初始化交易者
		endpoints,
		connections,
		txsRate,
		durationInt,
		txSize,
		allSahrd,
		relayrate,
		"broadcast_tx_"+broadcastTxMethod,
		datafile)

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
	var totaltxs int64
	var crosstxs int64
	totaltxs=0
	crosstxs=0
	var realtxs int64
	realtxs=0
	for _,v:=range rawdata{
		totaltxs +=int64(v.cnt)
		crosstxs+=int64(v.relaycnt)
	}

	for i, _ := range endpoints {
		client := clients[i]
		stats, err := calculateStatistics(
			client,
			initHeights[i],
			timeStart,
			durationInt,
			int64(txsRate),
		)
		if err != nil {
			printErrorAndExit(err.Error())
		}
		//在这里打印统计结果，说明已经统计完全，可以进行公式计算
		//计算totaltxs 与 crosstxs
		realtxs +=stats.TxsThroughput.Sum()
		// fmt.Println("交易数量",stats.TxsThroughput.Sum(),i)
		//printStatistics(stats, outputFormat)
	}
	var TPS float64
	TPS = float64(realtxs * totaltxs) / float64(totaltxs + crosstxs)

	var Crossrate float64
	Crossrate = float64(crosstxs) / float64(totaltxs)
	fmt.Println(TPS/float64(durationInt),Crossrate)
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


func CompareData(a int,b int) int {
	if a<=b{
		return a
	}else {
		return b
	}
}

func startTransacters(
	endpoints []string,
	connections,
	txsRate int,
	T       int,//发送交易持续时间
	txSize int,
	allshard []string,
	relayrate int,
	broadcastTxMethod string,
	datafile string) ([]*transacter, map[string] Stas){
	transacters := make([]*transacter, len(endpoints))

	count, data := CreateTx(txsRate*T, len(allshard), datafile)//传入allshard还是shardcount？
	iwg := sync.WaitGroup{}
	iwg.Add(len(endpoints))
	//使用sync.WaitGroup来创建与endpoints大小相同的线程
	for i, e := range endpoints {//为什么每个端口的transacter的shard都相同？
		flag := 0
		t := newTransacter(e, connections, CompareData(len(count[allshard[i]])/T,len(count[allshard[i]])), txSize, allshard[i], allshard, relayrate, count[allshard[i]], flag, broadcastTxMethod,T)//这一句是与每个端口间建立一个transacter
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
	return transacters,data
}
func printErrorAndExit(err string) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
