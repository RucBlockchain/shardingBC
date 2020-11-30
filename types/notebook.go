package types

import (
	"errors"
	"fmt"
	"github.com/tendermint/tendermint/libs/common"
	"time"
)

const (
	MsgNoteBookStop = byte(0x01)
)

// notebook的一条记录
// 对于普通节点而言 一个note保存一轮举报阶段的所有投票信息
// 对于拓扑链，一个note保存一个分片一轮的举报信息
type Note interface {
	Get()
	Set()
}

// 用来保存举报信息的抽象
// 有两个具体实现：普通节点和拓扑链节点，前者保存本片内的投票信息，后者保存不同分片的举报信息
type NoteBook interface {
	Cleanup() error        // 清空笔记本
	Add(interface{}) error // 添加一条记录
	//GetAll() []Note        // 返回当前笔记本里的所有记录
	Generate() []byte // 业务逻辑，根据目前已有的记录返回需要的总结数据
}

// -----------------------------------------------------------------------------
/* 此处为普通节点的notebook实现
 * 当共识失败时，就把该轮共识的投票信息记到notebook中
 * 业务层经过delta时间，调用notebook.generate，生成一份报告向拓扑链发送
 * thread-unsafe
 */
type Normalbook struct {
	delta int64 // 区块共识失败窗口 单位秒

	common.BaseService
	evidence    []VoteSet
	IsTicker    bool
	deltaTicker *time.Ticker
	statsMsg    chan byte
	sendCb      func(int) error
}

func (nb *Normalbook) Cleanup() error {
	nb.evidence = nb.evidence[0:0]
	nb.deltaTicker.Stop()
	nb.IsTicker = false
	return nil
}

func (nb *Normalbook) Add(note interface{}) error {
	voteset, ok := note.(VoteSet)
	if !ok {
		return errors.New("data type shoule be types.Voteset ")
	}

	if nb.IsTicker == false {
		nb.deltaTicker.Reset(time.Duration(nb.delta) * time.Second)
		nb.IsTicker = true
	}

	nb.evidence = append(nb.evidence, voteset)
	return nil
}

func (nb *Normalbook) Generate() []byte {
	return []byte("marshall voteset to tx")
}

func NewNormalBook(delta int64) *Normalbook {
	nb := &Normalbook{
		delta:       delta,
		evidence:    make([]VoteSet, 0, 100),
		deltaTicker: nil,
		statsMsg:    make(chan byte, 1),
	}
	return nb
}

func (nb *Normalbook) OnStart() error {
	// initialize private fields
	// start subroutines, etc.
	nb.deltaTicker = time.NewTicker(time.Duration(nb.delta) * time.Second)
	nb.IsTicker = true
	go func(tmp *Normalbook) {
		// 定时器任务 时间到了自动发送目前收集的证据
		for {
			select {
			case <-tmp.deltaTicker.C:
				fmt.Println("generate evidence: ", nb.Generate())
				fmt.Println("该发送证据了, ", nb.evidence)
			// TODO 实现发送函数
			//nb.sendCb(1) //调用发送回调 接口待定

			case s := <-tmp.statsMsg:
				if s == MsgNoteBookStop {
					// 退出信号
					break
				}
			}
		}
	}(nb)
	return nil
}

func (nb *Normalbook) OnStop() error {
	nb.statsMsg <- MsgNoteBookStop
	nb.deltaTicker.Stop()

	return nil
}
