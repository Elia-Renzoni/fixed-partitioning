package replication

import (
	"io"
	"net"
	"time"
)

func send(destinationAddress string, message []byte) []byte {
	conn, err := net.DialTimeout("tcp", destinationAddress, time.Duration(2*time.Second))
	if err != nil {
		return nil
	}

	conn.Write(message)

	var (
		buf  = make([]byte, 1048)
		n    int
		rErr error
	)
	for {
		n, rErr = conn.Read(buf)
		if rErr != nil && rErr == io.EOF {
			break
		}
	}

	if n <= 0 {
		return nil
	}

	return buf[:n]
}
