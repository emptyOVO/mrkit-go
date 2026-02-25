package master

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/emptyOVO/mrkit-go/rpc"
	"github.com/emptyOVO/mrkit-go/runtime/observability"
	"github.com/emptyOVO/mrkit-go/runtime/scheduler"
	"github.com/emptyOVO/mrkit-go/runtime/workerpool"
	log "github.com/sirupsen/logrus"
)

type Master struct {
	Workers      []WorkerInfo
	MapTasks     []MapTaskInfo
	ReduceTasks  []ReduceTaskInfo
	numWorkers   int
	totalWorkers int
	numReducer   int
	enoughWorker chan bool
	crashChan    chan string
	mux          sync.Mutex
	client       RpcClient
	registry     *workerpool.Registry
	metrics      *observability.Metrics
	rpc.UnimplementedMasterServer
}

func NewMaster(nWorker int, nReduce int) rpc.MasterServer {

	return &Master{
		ReduceTasks:  newReduceTasks(nReduce),
		numWorkers:   0,
		totalWorkers: nWorker,
		numReducer:   nReduce,
		client:       &workerClient{},
		registry:     workerpool.New(workerLeaseTTLFromEnv()),
		metrics:      observability.NewMetrics(),
		enoughWorker: make(chan bool, 1),
		crashChan:    make(chan string, 100),
	}
}

func workerLeaseTTLFromEnv() time.Duration {
	raw := os.Getenv("MR_WORKER_LEASE_TTL_SEC")
	if raw == "" {
		return 5 * time.Second
	}
	secs, err := strconv.Atoi(raw)
	if err != nil || secs <= 0 {
		return 5 * time.Second
	}
	return time.Duration(secs) * time.Second
}

// gRPC functions

func (ms *Master) WorkerRegister(ctx context.Context, in *rpc.WorkerInfo) (*rpc.RegisterResult, error) {
	var num int
	ms.mux.Lock()
	ms.Workers = append(ms.Workers, newWorker(in.Uuid, in.Ip))
	ms.numWorkers++
	if ms.registry != nil {
		ms.registry.Register(in.Uuid, in.Ip)
	}
	ms.updateWorkerAliveMetricLocked()

	num = ms.numWorkers
	ms.mux.Unlock()
	log.Info("[Master] Worker register success")
	return &rpc.RegisterResult{Result: true, Id: int64(num - 1)}, nil
}

func (ms *Master) UpdateIMDInfo(ctx context.Context, in *rpc.IMDInfo) (*rpc.UpdateResult, error) {
	ms.mux.Lock()
	for i, f := range in.Filenames {
		ms.ReduceTasks[i].IMDs = append(ms.ReduceTasks[i].IMDs,
			IMDInfo{
				IP:       ms.serviceDiscovey(in.Uuid),
				FileName: f,
			})
	}
	ms.mux.Unlock()
	log.Info(fmt.Sprintf("[Master] %v update IMD info success", in.Uuid))
	return &rpc.UpdateResult{Result: true}, nil
}

func (ms *Master) serviceDiscovey(uuid string) string {
	var ip string

	for i := range ms.Workers {
		if ms.Workers[i].UUID == uuid {
			ip = ms.Workers[i].getIP()
		}
	}

	return ip
}

// Normal Functions

func (ms *Master) waitForEnoughWorker() {
	log.Trace("[Master] Wait for enough workers")
	nWorker, totalWorker := ms.getWorkerNum()
	for nWorker < totalWorker {
		nWorker, totalWorker = ms.getWorkerNum()
	}
	log.Trace("[Master] Enough workers!")
}

func (ms *Master) getWorkerNum() (int, int) {
	ms.mux.Lock()
	nWorkers := ms.numWorkers
	tWorkers := ms.totalWorkers
	ms.mux.Unlock()
	return nWorkers, tWorkers
}

func (ms *Master) distributeWork(files []string) {
	log.Trace("[Master] Start distribute workload")
	numWorkers := ms.totalWorkers
	// Initialize MapTasks
	ms.MapTasks = newMapTasks(numWorkers)

	// Distribute work
	for _, file := range files {
		lineOffsets, err := fileLineOffsets(file)
		if err != nil {
			panic(err)
		}
		totalLine := len(lineOffsets) - 1
		baseWorkLoad := totalLine / numWorkers

		from := 0
		for i := 0; i < numWorkers; i++ {
			workLoad := baseWorkLoad
			if i < (totalLine % numWorkers) {
				workLoad++
			}

			// Use byte offsets directly so workers can seek without rescanning.
			startByte := int(lineOffsets[from])
			endByte := int(lineOffsets[from+workLoad])
			ms.MapTasks[i].addFile(file, startByte, endByte)
			from += workLoad
		}
	}
	log.Trace("[Master] End distribute workload")
}

func (ms *Master) availableWorkers(num int) ([]*WorkerInfo, int) {
	log.Info("[Master] Finding available workers to execute ", num, "/", len(ms.Workers))
	var retInfo []*WorkerInfo
	total := 0
	broken := 0

LOOP:
	for {
		// ms.mux.Lock()
		broken = 0
		for i := range ms.Workers {
			if total >= num {
				break LOOP
			}
			if ms.Workers[i].Health() {
				retInfo = append(retInfo, &ms.Workers[i])
				ms.setWorkerState(ms.Workers[i].UUID, WORKER_BUSY)
				total += 1
			} else if ms.Workers[i].Broken() {
				broken += 1
			}
		}
		if broken == len(ms.Workers) {
			log.Warn("No enough worker now, retrying...", total, num)
			time.Sleep(200 * time.Millisecond)
			continue
		}
		// ms.mux.Unlock()
	}

	retWorkers := len(retInfo)
	return retInfo, retWorkers
}

func lineNums(file string) int {
	f, err := os.Open(file)
	if err != nil {
		panic(err)
	}

	defer f.Close()

	scanner := bufio.NewScanner(f)

	num := 0
	for scanner.Scan() {
		num++
	}
	return num
}

func fileLineOffsets(file string) ([]int64, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	var offsets []int64
	offsets = append(offsets, 0)
	var cursor int64
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			cursor += int64(len(line))
			offsets = append(offsets, cursor)
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
	}
	return offsets, nil
}

func orDone(finish, crashChan <-chan string, numberOfTasks int) <-chan string {
	count := 0
	valStream := make(chan string)
	go func() {
		defer close(valStream)
		for {
			select {
			case <-finish:
				count += 1
				if count == numberOfTasks {
					return
				}
			case crashUUID, ok := <-crashChan:
				if !ok {
					if count != numberOfTasks {
						log.Panic("faild to distribute all Tasks")
						return
					}
				}

				valStream <- crashUUID

			}
		}
	}()
	return valStream
}

func (ms *Master) distributeMapTask() {
	log.Trace("[Master] Start Map task")
	if len(ms.MapTasks) == 0 {
		log.Trace("[Master] End Map task")
		return
	}
	stageStart := time.Now()

	stage := scheduler.New(schedulerRetryPolicyFromEnv())
	taskByID := make(map[string]*MapTaskInfo, len(ms.MapTasks))
	maxRetry := schedulerMaxRetryFromEnv()
	for i := range ms.MapTasks {
		taskID := fmt.Sprintf("map-%d", i)
		taskByID[taskID] = &ms.MapTasks[i]
		if err := stage.Submit(scheduler.Task{
			ID:       taskID,
			JobID:    "legacy",
			StageID:  "map",
			Type:     scheduler.TaskTypeMap,
			MaxRetry: maxRetry,
		}); err != nil {
			log.Panicf("submit map task failed: %v", err)
		}
		ms.incTaskTotal("map", "submitted")
	}

	total := len(ms.MapTasks)
	for {
		success, failed := schedulerStateCount(stage.Snapshot())
		if success == total {
			break
		}
		if failed > 0 {
			log.Panicf("map stage failed: %d/%d task(s) exhausted retries", failed, total)
		}

		workers, _ := ms.availableWorkers(1)
		worker := workers[0]
		task, err := stage.Assign(worker.UUID)
		if err == scheduler.ErrNoTask {
			ms.setWorkerState(worker.UUID, WORKER_IDLE)
			time.Sleep(50 * time.Millisecond)
			continue
		}
		if err != nil {
			ms.setWorkerState(worker.UUID, WORKER_IDLE)
			log.Warnf("[Master] map assign failed: %v", err)
			time.Sleep(50 * time.Millisecond)
			continue
		}
		payload, ok := taskByID[task.ID]
		if !ok {
			_ = stage.Fail(task.ID, "missing map payload")
			ms.setWorkerState(worker.UUID, WORKER_UNKNOWN)
			continue
		}
		payload.setState(TASK_INPROGRESS)

		go func(taskID string, w *WorkerInfo, mapTask *MapTaskInfo) {
			done := ms.client.Map(w.IP, mapTask.toRPC())
			if !done {
				mapTask.setState(TASK_IDLE)
				if err := stage.Fail(taskID, "map rpc failed"); err != nil {
					log.Warnf("[Master] map fail transition failed for %s: %v", taskID, err)
				}
				ms.observeRetryOrFail(stage, taskID, "map")
				ms.setWorkerState(w.UUID, WORKER_UNKNOWN)
				return
			}
			mapTask.setState(TASK_COMPLETED)
			if err := stage.Complete(taskID); err != nil {
				log.Warnf("[Master] map complete transition failed for %s: %v", taskID, err)
			}
			ms.incTaskTotal("map", "success")
			ms.setWorkerState(w.UUID, WORKER_IDLE)
		}(task.ID, worker, payload)
	}
	ms.observeStageDuration("map", time.Since(stageStart))

	log.Trace("[Master] End Map task")
}

func (ms *Master) distributeReduceTask() {
	log.Trace("[Master] Start Reduce task")
	if len(ms.ReduceTasks) == 0 {
		log.Trace("[Master] End Reduce task")
		return
	}
	stageStart := time.Now()

	stage := scheduler.New(schedulerRetryPolicyFromEnv())
	taskByID := make(map[string]*ReduceTaskInfo, len(ms.ReduceTasks))
	maxRetry := schedulerMaxRetryFromEnv()
	for i := range ms.ReduceTasks {
		taskID := fmt.Sprintf("reduce-%d", i)
		taskByID[taskID] = &ms.ReduceTasks[i]
		if err := stage.Submit(scheduler.Task{
			ID:       taskID,
			JobID:    "legacy",
			StageID:  "reduce",
			Type:     scheduler.TaskTypeReduce,
			MaxRetry: maxRetry,
		}); err != nil {
			log.Panicf("submit reduce task failed: %v", err)
		}
		ms.incTaskTotal("reduce", "submitted")
	}

	total := len(ms.ReduceTasks)
	for {
		success, failed := schedulerStateCount(stage.Snapshot())
		if success == total {
			break
		}
		if failed > 0 {
			log.Panicf("reduce stage failed: %d/%d task(s) exhausted retries", failed, total)
		}

		workers, _ := ms.availableWorkers(1)
		worker := workers[0]
		task, err := stage.Assign(worker.UUID)
		if err == scheduler.ErrNoTask {
			ms.setWorkerState(worker.UUID, WORKER_IDLE)
			time.Sleep(50 * time.Millisecond)
			continue
		}
		if err != nil {
			ms.setWorkerState(worker.UUID, WORKER_IDLE)
			log.Warnf("[Master] reduce assign failed: %v", err)
			time.Sleep(50 * time.Millisecond)
			continue
		}
		payload, ok := taskByID[task.ID]
		if !ok {
			_ = stage.Fail(task.ID, "missing reduce payload")
			ms.setWorkerState(worker.UUID, WORKER_UNKNOWN)
			continue
		}
		payload.SetState(TASK_INPROGRESS)

		go func(taskID string, w *WorkerInfo, reduceTask *ReduceTaskInfo) {
			done := ms.client.Reduce(w.IP, reduceTask.toRPC())
			if !done {
				reduceTask.SetState(TASK_IDLE)
				if err := stage.Fail(taskID, "reduce rpc failed"); err != nil {
					log.Warnf("[Master] reduce fail transition failed for %s: %v", taskID, err)
				}
				ms.observeRetryOrFail(stage, taskID, "reduce")
				ms.setWorkerState(w.UUID, WORKER_UNKNOWN)
				return
			}
			reduceTask.SetState(TASK_COMPLETED)
			if err := stage.Complete(taskID); err != nil {
				log.Warnf("[Master] reduce complete transition failed for %s: %v", taskID, err)
			}
			ms.incTaskTotal("reduce", "success")
			ms.setWorkerState(w.UUID, WORKER_IDLE)
		}(task.ID, worker, payload)
	}
	ms.observeStageDuration("reduce", time.Since(stageStart))

	log.Trace("[Master] End Reduce task")
}

func (ms *Master) endWorkers() {
	log.Trace("[Master] End Workers Start")
	for i := range ms.Workers {
		ms.client.End(ms.Workers[i].IP)
	}
	log.Trace("[Master] End Workers done")
}

func (ms *Master) setWorkerState(uuid string, state int) {
	ms.mux.Lock()
	defer ms.mux.Unlock()
	ms.setWorkerStateLocked(uuid, state)
}

func (ms *Master) setWorkerStateLocked(uuid string, state int) {
	for i := range ms.Workers {
		if ms.Workers[i].UUID != uuid {
			continue
		}
		ms.Workers[i].SetState(state)
		break
	}
	if ms.registry == nil {
		return
	}
	switch state {
	case WORKER_IDLE:
		ms.registry.SetState(uuid, workerpool.WorkerIdle)
	case WORKER_BUSY:
		ms.registry.SetState(uuid, workerpool.WorkerBusy)
	default:
		ms.registry.SetState(uuid, workerpool.WorkerUnknown)
	}
	ms.updateWorkerAliveMetricLocked()
}

func (ms *Master) metricsSnapshot() string {
	if ms.metrics == nil {
		return ""
	}
	return ms.metrics.RenderPrometheus()
}

func (ms *Master) incTaskTotal(stage, result string) {
	if ms.metrics == nil {
		return
	}
	ms.metrics.IncTaskTotal(stage, result)
}

func (ms *Master) observeRetryOrFail(stageScheduler *scheduler.Scheduler, taskID string, stageName string) {
	if ms.metrics == nil {
		return
	}
	switch schedulerTaskStateByID(stageScheduler.Snapshot(), taskID) {
	case scheduler.TaskPending:
		ms.metrics.IncTaskRetry(stageName)
	case scheduler.TaskFailed:
		ms.metrics.IncTaskTotal(stageName, "failed")
	}
}

func (ms *Master) observeStageDuration(stage string, d time.Duration) {
	if ms.metrics == nil {
		return
	}
	ms.metrics.ObserveStageDuration(stage, d)
}

func schedulerTaskStateByID(tasks []scheduler.Task, taskID string) scheduler.TaskState {
	for _, t := range tasks {
		if t.ID == taskID {
			return t.State
		}
	}
	return ""
}

func (ms *Master) updateWorkerAliveMetricLocked() {
	if ms.metrics == nil || ms.registry == nil {
		return
	}
	ms.metrics.SetWorkerAlive(len(ms.registry.Snapshot()))
}

func schedulerRetryPolicyFromEnv() scheduler.RetryPolicy {
	base := 200 * time.Millisecond
	max := 5 * time.Second
	if raw := os.Getenv("MR_TASK_RETRY_BASE_MS"); raw != "" {
		if ms, err := strconv.Atoi(raw); err == nil && ms > 0 {
			base = time.Duration(ms) * time.Millisecond
		}
	}
	if raw := os.Getenv("MR_TASK_RETRY_MAX_MS"); raw != "" {
		if ms, err := strconv.Atoi(raw); err == nil && ms > 0 {
			max = time.Duration(ms) * time.Millisecond
		}
	}
	if max < base {
		max = base
	}
	return scheduler.RetryPolicy{BaseDelay: base, MaxDelay: max}
}

func schedulerMaxRetryFromEnv() int {
	maxRetry := 8
	if raw := os.Getenv("MR_TASK_MAX_RETRY"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n >= 0 {
			maxRetry = n
		}
	}
	return maxRetry
}

func schedulerStateCount(tasks []scheduler.Task) (success int, failed int) {
	for _, t := range tasks {
		switch t.State {
		case scheduler.TaskSuccess:
			success++
		case scheduler.TaskFailed:
			failed++
		}
	}
	return success, failed
}
