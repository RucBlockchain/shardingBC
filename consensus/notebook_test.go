package consensus

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestNormalBook(t *testing.T) {
	delta := int64(1)
	t.Log("send delta: ", delta)
	nb := NewNormalBook(delta)

	assert.NotNil(t, nb)
	t.Log(nb)

	nb.OnStart()
	t.Log("normal notebook started. sleep 10s. You will see at least 9 sendInvokeMsg")

	time.Sleep(10 * time.Second)

	nb.OnStop()
	t.Log("stop notebook, you would see any sendMsg")
	time.Sleep(5)

	t.Log("end.")
}

func TestNormalbook_Cleanup(t *testing.T) {
	delta := int64(1)
	t.Log("send delta: ", delta)
	nb := NewNormalBook(delta)

	assert.NotNil(t, nb)
	t.Log(nb)

	nb.OnStart()
	t.Log("normal notebook started. sleep 10s. You will see at least 9 sendInvokeMsg")

	time.Sleep(10 * time.Second)
	start := time.Now()

	t.Log("notebook was trigged, you would see no sendMsg")
	// 模拟成功触发
	for time.Now().Sub(start) < 10*time.Second {
		nb.Trigger()
	}

	t.Log("notebook work again")

	time.Sleep(5 * time.Second)

	nb.OnStop()
	t.Log("end.")
}
