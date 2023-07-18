package component

import (
	"os"
)

func MaxInt64(x, y int64) int64 {
	if x > y {
		return x
	}
	return y
}

func GetSqlChannelProc() (*os.Process, error) {
	// sqlChannel pid is usually 1
	sqlChannelPid := os.Getppid()
	sqlChannelProc, err := os.FindProcess(sqlChannelPid)
	if err != nil {
		return nil, err
	}

	return sqlChannelProc, nil
}
