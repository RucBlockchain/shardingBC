package main

import (
	"encoding/json"
	"fmt"
	//"github.com/go-kit/kit/metrics"
	"math"
	"os"
	"time"

	"github.com/rcrowley/go-metrics"
	tmrpc "github.com/tendermint/tendermint/rpc/client"
	"github.com/tendermint/tendermint/types"
)

type statistics struct {
	TxsThroughput    metrics.Histogram `json:"txs_per_sec"`
	BlocksThroughput metrics.Histogram `json:"blocks_per_sec"`
}

// calculateStatistics calculates the tx / second, and blocks / second based
// off of the number the transactions and number of blocks that occurred from
// the start block, and the end time.
func calculateStatistics(
	client tmrpc.Client,
	minHeight int64,
	timeStart time.Time,
	duration int,
	plustx int64,
) (*statistics, error) {
		//fmt.Println(timeStart)
	timesub,_:=time.ParseDuration("-1s")
		timeStart=timeStart.Add(timesub)
	//timeStart=timeStart.Add(time.Second)
	timeEnd := timeStart.Add(time.Duration(duration) * time.Second)
	stats := &statistics{
		BlocksThroughput: metrics.NewHistogram(metrics.NewUniformSample(1000)),
		TxsThroughput:    metrics.NewHistogram(metrics.NewUniformSample(1000)),
	}

	var (
		numBlocksPerSec = make(map[int64]int64)
		numTxsPerSec    = make(map[int64]int64)
	)

	// because during some seconds blocks won't be created...
	for i := int64(0); i < int64(duration); i++ {
		numBlocksPerSec[i] = 0
		numTxsPerSec[i] = 0
	}

	blockMetas, err := getBlockMetas(client, minHeight, timeStart, timeEnd)
	if err != nil {
		return nil, err
	}
	sum := 0.0
	for _, blockMeta := range blockMetas {
		if blockMeta.Header.Time.Before(timeStart) {
			break
		}

		// check if block was created before timeEnd
		if blockMeta.Header.Time.After(timeEnd) {
			continue
		}


		sec := secondsSinceTimeStart(timeStart, blockMeta.Header.Time)

		// increase number of blocks for that second
		numBlocksPerSec[sec]++
		sum += float64(blockMeta.Header.NumTxs)
		// increase number of txs for that second
		numTxsPerSec[sec] += blockMeta.Header.NumTxs

		logger.Debug(fmt.Sprintf("%d txs at block height %d", blockMeta.Header.NumTxs, blockMeta.Header.Height))
	}
	//numTxsPerSec[0]+=plustx//为了添加init类型交易，现在不需要，应当删除

	for i := int64(0); i < int64(duration); i++ {
		stats.BlocksThroughput.Update(numBlocksPerSec[i])
		stats.TxsThroughput.Update(numTxsPerSec[i])
	}

	return stats, nil
}

func getBlockMetas(client tmrpc.Client, minHeight int64, timeStart, timeEnd time.Time) ([]*types.BlockMeta, error) {
	// get blocks between minHeight and last height
	// This returns max(minHeight,(last_height - 20)) to last_height
	info, err := client.BlockchainInfo(minHeight, 0)
	if err != nil {
		return nil, err
	}

	var (
		blockMetas = info.BlockMetas
		lastHeight = info.LastHeight
		diff       = lastHeight - minHeight
		offset     = len(blockMetas)
	)
	//fmt.Println("offset:", offset)
	for offset < int(diff) {
		// get blocks between minHeight and last height
		info, err := client.BlockchainInfo(minHeight, lastHeight-int64(offset))
		if err != nil {
			return nil, err
		}
		//fmt.Println("offset1:", offset)
		blockMetas = append(blockMetas, info.BlockMetas...)
		offset = len(blockMetas)
	}
	//fmt.Println("MinHeight:", minHeight, " LastHeight:", lastHeight)
	return blockMetas, nil
}

func secondsSinceTimeStart(timeStart, timePassed time.Time) int64 {
	return int64(math.Round(timePassed.Sub(timeStart).Seconds()))
}

func printStatistics(stats *statistics, outputFormat string) {
	if outputFormat == "json" {
		result, err := json.Marshal(struct {
			TxsThroughput    float64 `json:"txs_per_sec_avg"`
			BlocksThroughput float64 `json:"blocks_per_sec_avg"`
		}{stats.TxsThroughput.Mean(), stats.BlocksThroughput.Mean()})

		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(string(result))
	} else {
		fmt.Println(stats.TxsThroughput.Mean())

	}
}
