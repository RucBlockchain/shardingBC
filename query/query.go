package query

import "github.com/tendermint/tendermint/query/client"

type Query struct {
	ShardName string
	LeaderIp string
}
func (q *Query)QueryLeader(ShardName string)string{
	e:=client.NewEtcd()
	result := string(e.Query(ShardName))
	return result
}
func (q *Query)UpdateLeader(ShardName string,value string){
	e:=client.NewEtcd()
	e.Update(ShardName,value)
}
