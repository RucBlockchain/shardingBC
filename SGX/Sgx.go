package SGX

import (
	"math/rand"
	"time"
)

func GetCredibleTimeStamp()time.Time{
	CredibleTimeStamp:=time.Now()
	return CredibleTimeStamp
}
func GetCredibleRand(ShardCount int)int{
	r:=rand.New(rand.NewSource(time.Now().UnixNano()))
	time.Sleep(time.Millisecond)
	rnd:=r.Intn(ShardCount)
	return rnd
}
