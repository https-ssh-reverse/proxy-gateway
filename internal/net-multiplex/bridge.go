package net_multiplex

import (
	"io"
	"log"
	"net"
)

func BridgeConnectionSync(A net.Conn, aName string, B net.Conn, bName string) {
	go sync('A', 'B', A, aName, B, bName)
	sync('B', 'A', B, bName, A, aName)
}

func sync(l byte, r byte, left net.Conn, leftName string, right net.Conn, rightName string) {
	defer left.Close()
	defer right.Close()
	io.Copy(right, left)
	log.Printf("%c(%s:%s)->%c(%s:%s): EOF", l, leftName, left.RemoteAddr().String(), r, rightName, right.RemoteAddr().String())
}
