package utils

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"
)

func PidExists(pid int) (bool, error) {
	if pid <= 0 {
		return false, fmt.Errorf("invalid pid %v", pid)
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false, err
	}
	err = proc.Signal(syscall.Signal(0))
	if err == nil {
		return true, nil
	}
	if err.Error() == "os: process already finished" {
		return false, nil
	}
	errno, ok := err.(syscall.Errno)
	if !ok {
		return false, err
	}
	switch errno {
	case syscall.ESRCH:
		return false, nil
	case syscall.EPERM:
		return true, nil
	}
	return false, err
}

func GetPID() int {
	cmd := exec.Command("pgrep", "ffmpeg")
	out, _ := cmd.Output()
	parts := strings.Split(string(out), "\n")
	regexp.MustCompile(`[\r\n]+`).Split(parts[0], -1)
	pid, _ := strconv.Atoi(string([]byte(parts[0])))
	//fmt.Println(pid)
	return pid
}
