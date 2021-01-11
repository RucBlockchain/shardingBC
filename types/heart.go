package types

import "encoding/json"

type HeartType uint8

const (
	Normal  = HeartType(0x1) // 常规心跳消息
	special = HeartType(0x2) // 钦差心跳消息
	Report  = HeartType(0x3) // 举报心跳消息
)

type HeartMsg struct {
	Sign     []byte    //该轮共识的merkle_root对Content消息进行门限签名
	Shard_id string    //分片编号
	Node_id  string    //报告者片内编号
	Type     HeartType //心跳消息类型，分为3种：常规报告、钦差报告、举报
	Content  []byte    //报告内容，json格式，block header序列化对数据或是钦差报告
}

func NewHeartMsg(sign []byte,
	shardid, nodeid string,
	_type HeartType,
	headbyte []byte) HeartMsg {

	return HeartMsg{
		Sign:     sign,
		Shard_id: shardid,
		Node_id:  nodeid,
		Type:     _type,
		Content:  headbyte,
	}
}

func (msg *HeartMsg) Marshal() ([]byte, error) {
	return json.Marshal(msg)
}

func (msg *HeartMsg) Unmarshal(data []byte) error {
	err := json.Unmarshal(data, msg)
	return err
}

// 钦差报告内容
// TODO
type SpecialMsg struct {
	ProposeVS, VoteVS VoteSet
}

func (msg *SpecialMsg) Marshal() ([]byte, error) {
	return json.Marshal(msg)
}

func (msg *SpecialMsg) Unmarshal(data []byte) error {
	err := json.Unmarshal(data, msg)
	return err
}

// rstype参考consensus/types/round_state的RoundStepType
//// RoundStepType
//const (
//	RoundStepNewHeight     = RoundStepType(0x01) // Wait til CommitTime + timeoutCommit
//	RoundStepNewRound      = RoundStepType(0x02) // Setup new round and go to RoundStepPropose
//	RoundStepPropose       = RoundStepType(0x03) // Did propose, gossip proposal
//	RoundStepPrevote       = RoundStepType(0x04) // Did prevote, gossip prevotes
//	RoundStepPrevoteWait   = RoundStepType(0x05) // Did receive any +2/3 prevotes, start timeout
//	RoundStepPrecommit     = RoundStepType(0x06) // Did precommit, gossip precommits
//	RoundStepPrecommitWait = RoundStepType(0x07) // Did receive any +2/3 precommits, start timeout
//	RoundStepCommit        = RoundStepType(0x08) // Entered commit state machine
//)
func (msg *SpecialMsg) CollectInfo(rstype uint8, data interface{}) error {
	return nil
}
