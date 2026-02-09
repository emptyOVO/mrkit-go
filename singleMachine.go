package mapreduce

import (
	"sync"
)

var MasterIP string = ":10000"
var runtimeMu sync.Mutex

func StartSingleMachineJob(input []string, plugin string, nReducer int, nWorker int, inRAM bool) {
	if err := StartSingleMachineJobWithAddr(input, plugin, nReducer, nWorker, inRAM, MasterIP); err != nil {
		panic(err)
	}
}

func StartSingleMachineJobWithAddr(input []string, plugin string, nReducer int, nWorker int, inRAM bool, masterAddr string) error {
	if len(input) == 0 {
		return nil
	}
	runtimeMu.Lock()
	defer runtimeMu.Unlock()
	MasterIP = masterAddr
	return singleMachineJob(input, nWorker, nReducer, plugin, inRAM, masterAddr)
}

func singleMachineJob(input []string, nWorker int, nReducer int, plugin string, storeInRAM bool, masterAddr string) error {
	var wg sync.WaitGroup
	errCh := make(chan error, 2)

	wg.Add(1)
	go func() {
		if err := startMasterWithAddr(masterAddr, input, nWorker, nReducer); err != nil {
			errCh <- err
		}
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		if err := startSingleMachineWorkerWithMaster(masterAddr, plugin, nWorker, nReducer, storeInRAM); err != nil {
			errCh <- err
		}
		wg.Done()
	}()

	wg.Wait()
	close(errCh)
	for err := range errCh {
		return err
	}
	return nil
}
