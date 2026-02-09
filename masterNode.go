package mapreduce

func StartMaster(input []string, plugin string, nReducer int, nWorker int, inRAM bool) {
	if err := StartMasterWithAddr(input, plugin, nReducer, nWorker, inRAM, MasterIP); err != nil {
		panic(err)
	}
}

func StartMasterWithAddr(input []string, plugin string, nReducer int, nWorker int, inRAM bool, masterAddr string) error {
	return startMasterWithAddr(masterAddr, input, nWorker, nReducer)
}
