package identypes

import "syscall"

func getShard() string {
	v, _ := syscall.Getenv("TASKID")
	return v
}

// 返回该节点是否是钦差节点
// 依据是否有MONITOR环境变量且值为1
func IsMonitor() bool {
	if v, ok := syscall.Getenv("Monitor"); !ok {
		return false
	} else if v == "TRUE" {
		return true
	}
	return false
}
