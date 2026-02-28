package replication

import (
	"net"
	"time"
)

func Send(destinationAddress string, message []byte) []byte {
	conn, err := net.DialTimeout("tcp", destinationAddress, time.Duration(2*time.Second))
	if err != nil {
		return nil
	}

	conn.Write(message)

	var (
		buf  = make([]byte, 2048)
		n    int
		rErr error
	)

	n, rErr = conn.Read(buf)
	if rErr != nil {
		return nil
	}
	return buf[:n]
}
