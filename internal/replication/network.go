package replication

import (
	"net"
	"time"
)

func Send(destinationAddress string, message []byte) []byte {
	conn, err := net.DialTimeout("tcp", destinationAddress, 2*time.Second)
	if err != nil {
		return nil
	}
	defer conn.Close()

	conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	_, err = conn.Write(message)
	if err != nil {
		return nil
	}

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