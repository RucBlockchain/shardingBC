package SGX

import (
	"fmt"
	"testing"
)

func TestGetCredibleRand(t *testing.T) {
	rnd := GetCredibleRand(64)
	fmt.Println(rnd)
	rnd1 := GetCredibleRand(64)
	fmt.Println(rnd1)
}
func TestGetCredibleTimeStamp(t *testing.T) {
	time := GetCredibleTimeStamp()
	fmt.Println(time)
}
