package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/emptyOVO/mrkit-go/rpc"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type RpcClient interface {
	//Connect()
	WorkerRegister(w *rpc.WorkerInfo) (int, error)
	UpdateIMDInfo(u *rpc.IMDInfo) bool
	GetIMDData(ip string, filename string) []KV
}

type masterClient struct {
	master rpc.MasterClient
	conn   *grpc.ClientConn
}

func Connect(ip string) (*grpc.ClientConn, rpc.MasterClient) {
	conn, err := grpc.Dial(ip, grpc.WithInsecure())
	if err != nil {
		log.Warn(err)
	}
	return conn, rpc.NewMasterClient(conn)
}

func (client *masterClient) WorkerRegister(w *rpc.WorkerInfo) (int, error) {
	const (
		maxAttempts = 40
		backoff     = 200 * time.Millisecond
	)
	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		log.Trace("With Time out")
		log.Trace("Start RPC call")
		r, err := client.master.WorkerRegister(ctx, w)
		cancel()
		log.Trace("End RPC call")
		if err != nil {
			if respErr, ok := status.FromError(err); ok {
				lastErr = fmt.Errorf("register worker rpc failed: %s", respErr.Message())
				if respErr.Code() != codes.Unavailable && respErr.Code() != codes.DeadlineExceeded {
					return 0, lastErr
				}
			} else {
				lastErr = err
			}
			time.Sleep(backoff)
			continue
		}
		if !r.Result {
			lastErr = fmt.Errorf("register worker rpc returned false")
			time.Sleep(backoff)
			continue
		}
		return int(r.Id), nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("register worker rpc failed after retries")
	}
	return 0, lastErr
}

func (client *masterClient) UpdateIMDInfo(u *rpc.IMDInfo) bool {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	r, err := client.master.UpdateIMDInfo(ctx, u)
	if err != nil {
		respErr, ok := status.FromError(err)
		if ok {
			//actual error from gRPC
			//todo: we can improve error handling by using grpc statusCodes()
			log.Panic(respErr.Message())
		} else {
			log.Panic(err)
		}
		return false
	}

	if !r.Result {
		log.Panic("Update IMD Info Error")
	}

	return r.Result
}

func (client *masterClient) GetIMDData(ip string, filename string) []KV {
	conn, _ := Connect(ip)

	c := rpc.NewWorkerClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	r, err := c.GetIMDData(ctx, &rpc.IMDLoc{
		Filename: filename,
	})
	if err != nil {
		respErr, ok := status.FromError(err)
		if ok {
			//actual error from gRPC
			//todo: we can improve error handling by using grpc statusCodes()
			log.Panic(respErr.Message())
		} else {
			log.Panic(err)
		}
	}

	return decodeIMDKVs(r.Kvs)
}
