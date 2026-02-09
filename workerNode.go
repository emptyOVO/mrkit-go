package mapreduce

func StartWorker(input []string, plugin string, nReducer int, nWorker int, storeInRAM bool) {
	if err := StartWorkerWithAddr(input, plugin, nReducer, nWorker, storeInRAM, MasterIP); err != nil {
		panic(err)
	}
}

func StartWorkerWithAddr(input []string, plugin string, nReducer int, nWorker int, storeInRAM bool, masterAddr string) error {
	return startWorkerWithMaster(masterAddr, plugin, nWorker, nReducer, storeInRAM)
}
