package identypes

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCrossMessageFromByteSlices(t *testing.T) {
	expected := NewCrossMessage(nil, []byte{0}, []byte{1}, []byte{2}, "tree path", "A", "B", 10)
	cmbytes := expected.Data()

	assert.NotNil(t, cmbytes, "CrossMessage序列化错误")

	actual, err := CrossMessageFromByteSlices(cmbytes)
	assert.Nil(t, err, "CrossMessage反序列化失败, err: ", err)

	t.Log("expected: ", expected)
	t.Log("actual: ", actual)
}
