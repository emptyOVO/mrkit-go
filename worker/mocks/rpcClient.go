package mocks

import (
	"github.com/emptyOVO/mrkit-go/rpc"
	"google.golang.org/grpc"
)

type MasterClient struct {
	master rpc.MasterClient
	conn   *grpc.ClientConn
}

var Request interface{}
var Result interface{}
var Counter int

func Connect() (*grpc.ClientConn, rpc.MasterClient) {
	conn := grpc.ClientConn{}

	return &conn, rpc.NewMasterClient(&conn)
}

func (client *MasterClient) WorkerRegister(w *rpc.WorkerInfo) (int, error) {
	Request = w
	return Result.(int), nil
}

func (client *MasterClient) UpdateIMDInfo(u *rpc.IMDInfo) bool {
	Request = u
	return Result.(bool)
}

func (client *MasterClient) GetIMDData(ip string, filename string) []rpc.KV {
	return Result.([]rpc.KV)
}
