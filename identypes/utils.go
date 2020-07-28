package identypes

import "syscall"

func getShard() string {

	v, _ := syscall.Getenv("TASKID")
	return v
}
