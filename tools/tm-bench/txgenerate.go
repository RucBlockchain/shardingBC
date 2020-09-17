package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/md5"
	crand "crypto/rand"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"math/big"
	"os"
	"strconv"
	"time"
	tp "github.com/tendermint/tendermint/identypes"
)
var Count map[string] *ecdsa.PrivateKey


type Stas struct {
	cnt int //交易总数
	relaycnt int //跨片数量
	relayrate	float64 //跨片比例
}
//allshard 统计string的长度，分片数量是多少 现实情况 跨片 本片交易融合在一起
//-t * -T


type ecdsaSignature struct {
	X, Y *big.Int
}
func (s Stas) GetRelayrate() float64 {//得到一个分片的跨片比例
	return float64(s.relaycnt) / float64(s.cnt)
}
func toShard(pubKey string, shdCnt int) string {//把传入的公钥字符串转换为大整数后对shdCnt取模
	sender, err := new(big.Int).SetString(pubKey, 16)
	if !err{
		log.Fatalln("When getting the big int of sender", err)
	}
	shd := big.NewInt(int64(shdCnt))
	mod := new(big.Int)
	_, mod = sender.DivMod(sender, shd, mod)

	return mod.String()
}
func pub2string(pub ecdsa.PublicKey) string {

	coor := ecdsaSignature{X: pub.X, Y: pub.Y}
	b, _ := asn1.Marshal(coor)

	return hex.EncodeToString(b)
}
//文件
func createCount(name string) *ecdsa.PrivateKey {

	priv, _ := ecdsa.GenerateKey(elliptic.P256(), crand.Reader)
	return priv
}
func digest(Content string) []byte {
	origin := []byte(Content)

	// 生成md5 hash值
	digest_md5 := md5.New()
	digest_md5.Write(origin)

	return digest_md5.Sum(nil)
}
func bigint2str(r, s big.Int) string {
	coor := ecdsaSignature{X: &r, Y: &s}
	b, _ := asn1.Marshal(coor)
	return hex.EncodeToString(b)
}

//从数据集中取出钱txcount条交易，有shardcount个分片
func CreateTx(txcount int,shardcount int, datafile string) (map[string] [][]byte, map[string] Stas){
	var txs map[string] [][]byte
	txs = make(map[string] [][]byte,txcount)
	var txdata map[string] Stas
	txdata = make(map[string] Stas,shardcount)
	for i := 0; i < shardcount; i++ {
		txdata[strconv.Itoa(i)] = Stas{
			cnt:       0,
			relaycnt:  0,
			relayrate: 0,
		}
	}
	cnt := 0  //用于记录有多少个交易产生
	//open file
	//Count应该开多大还需要确定
	Count=make(map[string] *ecdsa.PrivateKey,100000)
	csvfile, err := os.Open(datafile)
	if err != nil {
		log.Fatalln("Couldn't open the csv file", err)
	}
	defer csvfile.Close()	//这句意思是在return前关闭文件

	// parse the file
	r := csv.NewReader(csvfile)

	// Iterate through the records
	count:=0

	for {
		// Read each record from csv
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if count==0{
			count+=1
			continue
		}
		count+=1
		if err != nil {
			log.Fatal(err)
		}
		from := record[1]
		//to := record[5]
		if Count[record[1]]==nil{
			Count[record[1]]=createCount(record[4])
		}
		if Count[record[2]]==nil{
			Count[record[2]]=createCount(record[5])
		}
		record[1]=pub2string(Count[record[1]].PublicKey)
		record[2]=pub2string(Count[record[2]].PublicKey)
		//merge to one string of content
		var content bytes.Buffer
		var idx = [3]int {1,2,3}
		for k, i := range idx {
			content.WriteString(record[i])
			if k != 3 {
				content.WriteString("_")
			}
		}
		tx_content:=content.String()+strconv.FormatInt(time.Now().UnixNano(), 10)
		tr, ts, _ := ecdsa.Sign(crand.Reader, Count[from], digest(tx_content))
		sig := bigint2str(*tr, *ts)
		id:=sha256.Sum256([]byte(tx_content))

		//assign every attribute of Tx
		var temp tp.TX
		temp.Sender = toShard(record[1], shardcount)
		temp.Receiver = toShard(record[2], shardcount)
		temp.Content = tx_content
		temp.ID = id
		temp.TxSignature = sig
		if temp.Sender == temp.Receiver{
			temp.Txtype = "tx"
		}else {
			temp.Txtype = "relaytx"
		}

		var res []byte
		res, _ = json.Marshal(temp)
		txs[temp.Sender] = append(txs[temp.Sender], res)
		//该交易所属

		num := txdata[temp.Sender]
		num.cnt += 1
		if temp.Txtype == "relaytx" {
			num.relaycnt += 1
		}
		txdata[temp.Sender] = num


		//txs = append(txs, temp)
		cnt++
		if cnt == txcount {
			break
		}
	}
	return txs,txdata
}
