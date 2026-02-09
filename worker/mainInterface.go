package worker

import (
	"net"
	"os"
	"plugin"
	"time"

	"github.com/emptyOVO/mrkit-go/rpc"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

var MasterIP string

func Init(masterIP string) {
	MasterIP = masterIP
	log.SetLevel(log.TraceLevel)
}

func StartWorker(pluginFile string, nReduce int, addr string, storeInRAM bool) {
	// start gRPC server
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Panic(err)
	}
	wr := newWorker(nReduce, storeInRAM)
	workerStruct := wr.(*Worker)
	baseServer := grpc.NewServer()
	rpc.RegisterWorkerServer(baseServer, wr)
	go func() {
		if err := baseServer.Serve(listener); err != nil {
			log.Error(err)
		}
	}()
	log.Info("Worker gRPC server start")

	workerStruct.Mapf, workerStruct.Reducef = loadPlugin(pluginFile)
	log.Info("Worker load plugin finish")

	// Register itself
	id, err := workerStruct.Client.WorkerRegister(&rpc.WorkerInfo{
		Uuid: workerStruct.UUID,
		Ip:   addr,
	})
	if err != nil {
		log.Panic(err)
	}
	workerStruct.setID(id)
	log.Info("Worker register itself finish")

	defer workerStruct.Client.(*masterClient).conn.Close()

	<-workerStruct.EndChan

	// Sleep for a while for waiting the End Grpc response sent to master
	time.Sleep(500 * time.Millisecond)
	baseServer.Stop()
}

// load the application Map and Reduce functions
// from a plugin file, e.g. .so files
func loadPlugin(filename string) (func(string, string, MrContext), func(string, []string, MrContext)) {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		log.Panic(err)
	}
	p, err := plugin.Open(filename)
	if err != nil {
		log.Panic(err)
	}
	xmapf, err := p.Lookup("Map")
	if err != nil {
		log.Panic(err)
	}
	mapf := xmapf.(func(string, string, MrContext))
	xreducef, err := p.Lookup("Reduce")
	if err != nil {
		log.Panic(err)
	}
	reducef := xreducef.(func(string, []string, MrContext))

	return mapf, reducef
}
