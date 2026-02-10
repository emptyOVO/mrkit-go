package worker

import (
	"context"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/emptyOVO/mrkit-go/rpc"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

type MapFormat (func(string, string, MrContext))
type ReduceFormat (func(string, []string, MrContext))

type Worker struct {
	UUID       string
	ID         int
	nReduce    int
	Mapf       MapFormat
	Reducef    ReduceFormat
	Chan       MrContext
	EndChan    chan bool
	storeInRAM bool
	State      rpc.WorkerState_State
	Client     RpcClient
	mux        sync.Mutex
	rpc.UnimplementedWorkerServer
}

func newWorker(nReduce int, inRAM bool) rpc.WorkerServer {
	conn, master := Connect(MasterIP)
	return &Worker{
		UUID:       uuid.New().String(),
		nReduce:    nReduce,
		Chan:       newMrContext(),
		EndChan:    make(chan bool),
		Client:     &masterClient{master: master, conn: conn},
		storeInRAM: inRAM,
		State:      rpc.WorkerState_IDLE,
	}
}

// gRPC functions

func (wr *Worker) Map(ctx context.Context, in *rpc.MapInfo) (*rpc.Result, error) {
	log.Info("[Worker] Start Map")

	wr.setWorkerState(rpc.WorkerState_BUSY)

	log.Trace("[Worker] Start Mapping")
	done := make(chan int, 100)
	mapChan := newMrContext()
	for _, fInfo := range in.Files {
		content := partialContent(fInfo)
		go func(f0 *rpc.MapFileInfo, c0 string) {
			wr.Mapf(f0.FileName, c0, mapChan)
			done <- 1
		}(fInfo, content)
	}
	log.Trace("[Worker] Finish Mapping")

	imdKV := make([][]KV, wr.nReduce)

	// Get intermediate KV
	// Partition result into R piece
	log.Trace("[Worker] Start partition intermediate kv")
	count := 0

LOOP:
	for {
		select {
		case mapKV, haveKV := <-mapChan.Chan:
			if haveKV {
				reducerID := reducerForKey(mapKV.Key, wr.nReduce)
				imdKV[reducerID] = append(imdKV[reducerID], mapKV)
			} else {
				break LOOP
			}

		case <-done:
			count++
			if count == len(in.Files) {
				close(mapChan.Chan)
			}
		}

	}
	log.Trace("[Worker] End partition intermediate kv")

	log.Trace("[Worker] Write intermediate kv to file")
	filenames := writeIMDToLocalFile(imdKV, wr.UUID, wr.storeInRAM)
	log.Trace("[Worker] End Write intermediate kv to file")

	log.Trace("[Worker] Tell Master the intermediate info")
	// Return to the Master
	wr.Client.UpdateIMDInfo(&rpc.IMDInfo{
		Uuid:      wr.UUID,
		Filenames: filenames,
	})
	log.Trace("[Worker] Finish Tell Master the intermediate info")
	log.Info("[Worker] Finish Map Task")
	wr.setWorkerState(rpc.WorkerState_IDLE)

	return &rpc.Result{Result: true}, nil
}

func partialContent(fInfo *rpc.MapFileInfo) string {
	f, err := os.Open(fInfo.FileName)
	if err != nil {
		panic(err)
	}

	defer f.Close()

	start := fInfo.From
	end := fInfo.To
	if end < start {
		return ""
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		panic(err)
	}
	size := end - start
	if size <= 0 {
		return ""
	}
	buf := make([]byte, size)
	_, err = io.ReadFull(f, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		panic(err)
	}
	return string(buf)
}

func writeIMDToLocalFile(imdKV [][]KV, uuid string, inRAM bool) []string {
	// Filenames must stay aligned with reducer index, otherwise master will
	// dispatch wrong partitions to reducers and produce duplicate outputs.
	filenames := make([]string, len(imdKV))
	var wg sync.WaitGroup
	for taskID, kvs := range imdKV {
		wg.Add(1)
		go func(t int, s []KV) {
			defer wg.Done()
			filenames[t] = writeIMDToLocalFileParallel(t, s, uuid, inRAM)
		}(taskID, kvs)
	}
	wg.Wait()
	return filenames
}

func reducerForKey(key string, nReduce int) int {
	if nReduce <= 0 {
		panic("nReduce must be > 0")
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return int(h.Sum32()&0x7fffffff) % nReduce
}

func writeIMDToLocalFileParallel(taskId int, kvs []KV, uuid string, inRAM bool) string {
	content := encodeIMDKVs(kvs)

	var fname string
	if inRAM {
		baseDir := "/dev/shm"
		if info, err := os.Stat(baseDir); err != nil || !info.IsDir() {
			baseDir = os.TempDir()
		}
		fname = filepath.Join(baseDir, fmt.Sprintf("imd-%v-%v.txt", uuid, taskId))
	} else {
		fname = fmt.Sprintf("output/imd-%v-%v.txt", uuid, taskId)
	}
	if err := os.MkdirAll(filepath.Dir(fname), 0o755); err != nil {
		panic(err)
	}
	file, err := os.Create(fname)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	file.WriteString(content)
	return fname
}

func (wr *Worker) Reduce(ctx context.Context, in *rpc.ReduceInfo) (*rpc.Result, error) {
	log.Info("[Worker] Start Reduce")

	wr.setWorkerState(rpc.WorkerState_BUSY)

	log.Trace("[Worker] Get intermediate file")
	var imdKVs []KV
	for _, fInfo := range in.Files {
		imdKVs = append(imdKVs, wr.Client.GetIMDData(fInfo.Ip, fInfo.Filename)...)
	}

	log.Trace("[Worker] Sort intermediate KV")
	// Sort
	sort.Sort(byKey(imdKVs))

	outputFile := fmt.Sprintf("mr-out-%v.txt", wr.ID)
	ofile, _ := os.Create(outputFile)

	log.Trace("[Worker] Start Reducing")
	// Reduce all the intermediate KV
	reduceChan := newMrContext()
	i := 0
	for i < len(imdKVs) {
		j := i + 1
		for j < len(imdKVs) && imdKVs[j].Key == imdKVs[i].Key {
			j++
		}
		values := []string{}
		for k := i; k < j; k++ {
			values = append(values, imdKVs[k].Value)
		}
		wr.Reducef(imdKVs[i].Key, values, reduceChan)

		output := <-reduceChan.Chan

		fmt.Fprintf(ofile, "%v %v\n", output.Key, output.Value)

		i = j
	}
	log.Trace("[Worker] End Reducing")
	log.Info("[Worker] End Reduce")
	wr.setWorkerState(rpc.WorkerState_IDLE)

	return &rpc.Result{Result: true}, nil
}

func (wr *Worker) GetIMDData(ctx context.Context, in *rpc.IMDLoc) (*rpc.JSONKVs, error) {
	log.Info("[Worker] RPC Get intermediate file")
	return &rpc.JSONKVs{
		Kvs: generateIMDKV(in.Filename),
	}, nil
}

func generateIMDKV(file string) string {
	b, err := os.ReadFile(file)
	if err != nil {
		panic(err)
	}

	return strings.TrimSpace(string(b))
}

func (wr *Worker) End(ctx context.Context, in *rpc.Empty) (*rpc.Empty, error) {
	log.Info("[Worker] End worker")
	wr.EndChan <- true
	return &rpc.Empty{}, nil
}

func (wr *Worker) setID(id int) {
	wr.ID = id
}

func (wr *Worker) Health(ctx context.Context, in *rpc.Empty) (*rpc.WorkerState, error) {
	log.Trace("[Worker] Health Check")

	wr.mux.Lock()
	state := wr.State
	wr.mux.Unlock()

	return &rpc.WorkerState{State: state}, nil
}

func (wr *Worker) setWorkerState(state rpc.WorkerState_State) {
	wr.mux.Lock()
	wr.State = state
	wr.mux.Unlock()
}
