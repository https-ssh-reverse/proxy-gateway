package client

import (
	"net"
	_ "github.com/gongt/proxy-gateway/internal/net-multiplex"
	"github.com/hashicorp/yamux"
	"log"
	"github.com/gongt/proxy-gateway/api"
	"google.golang.org/grpc"
	"time"
	"context"
	"encoding/binary"
	"fmt"
	"github.com/gongt/proxy-gateway/internal/net-multiplex"
)

type MultiplexClient struct {
	client  bridge_api_call.ConnectionBridgeClient
	mapper  map[uint32]*net_multiplex.NaiveAddr
	session *yamux.Session
}

func NewMultiplexClient(conn net.Conn) *MultiplexClient {
	session, err := yamux.Client(conn, nil)
	if err != nil {
		log.Fatal("create mux client failed: ", err)
	}

	rpcChannel, err := grpc.Dial(session.Addr().String(), grpc.WithInsecure(), grpc.WithDialer(func(s string, duration time.Duration) (net.Conn, error) {
		return session.Open()
	}))
	if err != nil {
		log.Fatal("create rpc channel failed: ", err)
	}

	client := bridge_api_call.NewConnectionBridgeClient(rpcChannel)

	return &MultiplexClient{
		client:  client,
		mapper:  make(map[uint32]*net_multiplex.NaiveAddr),
		session: session,
	}
}

func (m *MultiplexClient) OpenTCP(remote string, connect *net_multiplex.NaiveAddr) uint32 {
	typeId, err := m.client.OpenTCP(context.Background(), &bridge_api_call.OpenMessage{Address: remote})
	if err != nil {
		log.Fatal(err)
	}
	m.mapper[typeId.Id] = connect
	return typeId.Id
}

func (m *MultiplexClient) OpenUnix(remote string, connect *net_multiplex.NaiveAddr) uint32 {
	typeId, err := m.client.OpenUnix(context.Background(), &bridge_api_call.OpenMessage{Address: remote})
	if err != nil {
		log.Fatal(err)
	}
	m.mapper[typeId.Id] = connect
	return typeId.Id
}

func (m *MultiplexClient) EventLoop() {
	go func() {
		<-m.session.CloseChan()
		log.Fatal("Error: connection dropped by server.")
	}()

	for {
		conn, err := m.session.Accept()
		if err != nil {
			log.Fatal("can not accept connection: ", err)
		}
		log.Println("got new connection from server.")

		go m.handle(conn)
	}
}

func (m *MultiplexClient) handle(conn net.Conn) {
	defer conn.Close()

	var id uint32

	err := binary.Read(conn, binary.LittleEndian, &id)
	log.Printf("<<< %v", id)
	if err != nil {
		duplicateMessage(conn, "read typeId failed:", err)
		return
	}

	localConnect, ok := m.mapper[id]
	if !ok {
		duplicateMessage(conn, "invalid typeId:", id)
		return
	}

	log.Printf("type id is %d, connecting to %s\n", id, localConnect.String())

	t, err := net_multiplex.Dial(localConnect, 10*time.Second)
	if err != nil {
		log.Println("failed to connect local:", err)
		fmt.Fprintln(conn, "target connection to %s failed: %s.\n", localConnect.String(), err.Error())
		return
	}

	log.Println("local connected, bridge has start.")
	net_multiplex.BridgeConnectionSync(t, "local", conn, "remote")
}
func duplicateMessage(conn net.Conn, s ...interface{}) {
	fmt.Fprintln(conn, s...)
	log.Println(s...)
}
