package main

import "testing"

func TestTransacter_Start(tt *testing.T) {
	//func newTransacter(target string, connections, rate int, size int, shard string, allshard []string, relayrate int, count []plist, flag int, broadcastTxMethod string) *transacter {

	count := createCount([]string{"1", "2", "3"}, 1)

	t := newTransacter("1", 0, 0, 0, "3", []string{"1", "2", "3"}, 1, count, 1, "broadcast_tx_")
	send_shard := deleteSlice(t.allshard, t.shard)

	for i := 0; i < 1000; i++ {
		ntx := t.updateTx(i, send_shard, t.shard, 1, 1000)
		tt.Log(ntx)
	}

}
