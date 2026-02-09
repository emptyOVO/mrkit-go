package mapreduce

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/emptyOVO/mrkit-go/master"
	"github.com/emptyOVO/mrkit-go/worker"
	"github.com/spf13/cobra"
)

func ParseArg() ([]string, string, int, int, bool) {
	var files []string
	var nReducer int64
	var nWorker int64
	var plugin string
	var inRAM bool
	var port int64
	var rootCmd = &cobra.Command{
		Use:   "mapreduce",
		Short: "MapReduce is an easy-to-use parallel framework by Bo-Wei Chen(BWbwchen)",
		Long: `MapReudce is an easy-to-use Map Reduce Go parallel-computing framework inspired by 2021 6.824 lab1.
It supports multiple workers threads on a single machine and multiple processes on a single machine right now.`,
		Run: func(cmd *cobra.Command, args []string) {
			tempFiles := []string{}

			for _, f := range files {
				// expand the file path
				expandFiles, err := filepath.Glob(f)
				if err != nil {
					panic(err)
				}
				tempFiles = append(tempFiles, expandFiles...)
			}

			pluginFiles, err := filepath.Glob(plugin)
			if err != nil {
				panic(err)
			} else if len(pluginFiles) == 0 {
				panic("No such file")
			}

			plugin = pluginFiles[0]
			files = tempFiles
			MasterIP = ":" + strconv.Itoa(int(port))
		},
	}

	rootCmd.PersistentFlags().StringSliceVarP(&files, "input", "i", []string{}, "Input files")
	rootCmd.MarkPersistentFlagRequired("input")
	rootCmd.PersistentFlags().StringVarP(&plugin, "plugin", "p", "", "Plugin .so file")
	rootCmd.MarkPersistentFlagRequired("plugin")
	rootCmd.PersistentFlags().Int64VarP(&nReducer, "reduce", "r", 1, "Number of Reducers")
	rootCmd.PersistentFlags().Int64VarP(&nWorker, "worker", "w", 4, "Number of Workers(for master node)\nID of worker(for worker node)")
	rootCmd.PersistentFlags().Int64Var(&port, "port", 10000, "Port number")
	rootCmd.PersistentFlags().BoolVarP(&inRAM, "inRAM", "m", true, "Whether write the intermediate file in RAM")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	return files, plugin, int(nReducer), int(nWorker), inRAM
}

func startSingleMachineWorker(plugin string, nWorker int, nReducer int, storeInRAM bool) {
	if err := startSingleMachineWorkerWithMaster(MasterIP, plugin, nWorker, nReducer, storeInRAM); err != nil {
		panic(err)
	}
}

func startSingleMachineWorkerWithMaster(masterAddr string, plugin string, nWorker int, nReducer int, storeInRAM bool) error {
	if nWorker < nReducer {
		return fmt.Errorf("Need more worker!")
	}

	pluginFile, _ := filepath.Abs(plugin)

	var wg sync.WaitGroup
	worker.Init(masterAddr)
	basePort := masterPort(masterAddr)
	errCh := make(chan error, nWorker)

	// Start Worker
	for i := 0; i < nWorker; i++ {
		wg.Add(1)
		go func(i0 int) {
			defer wg.Done()
			// Keep each worker on a disjoint candidate sequence to avoid collisions.
			start := basePort + i0 + 1
			if err := startWorkerWithRetryE(pluginFile, nReducer, start, nWorker, storeInRAM); err != nil {
				errCh <- err
			}
		}(i)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		return err
	}
	return nil
}

func startMaster(input []string, nWorker int, nReducer int) {
	if err := startMasterWithAddr(MasterIP, input, nWorker, nReducer); err != nil {
		panic(err)
	}
}

func startMasterWithAddr(masterAddr string, input []string, nWorker int, nReducer int) error {
	inputFiles := []string{}
	for _, s := range input {
		f, _ := filepath.Abs(s)
		inputFiles = append(inputFiles, f)
	}

	var wg sync.WaitGroup
	// Start master
	// master.StartMaster(os.Args[1:], nReducer, MasterIP)
	wg.Add(1)
	go func() {
		master.StartMaster(inputFiles, nWorker, nReducer, masterAddr)
		wg.Done()
	}()

	wg.Wait()
	return nil
}

func startWorker(plugin string, id int, nReducer int, storeInRAM bool) {
	if err := startWorkerWithMaster(MasterIP, plugin, id, nReducer, storeInRAM); err != nil {
		panic(err)
	}
}

func startWorkerWithMaster(masterAddr string, plugin string, id int, nReducer int, storeInRAM bool) error {
	pluginFile, _ := filepath.Abs(plugin)

	var wg sync.WaitGroup
	worker.Init(masterAddr)
	basePort := masterPort(masterAddr)
	var runErr error

	// Start Worker
	wg.Add(1)
	go func() {
		defer wg.Done()
		start := basePort + id + 1
		runErr = startWorkerWithRetryE(pluginFile, nReducer, start, 1, storeInRAM)
	}()

	wg.Wait()
	return runErr
}

func masterPort(masterAddr string) int {
	raw := strings.TrimSpace(masterAddr)
	if raw == "" {
		return 10000
	}
	parts := strings.Split(raw, ":")
	last := strings.TrimSpace(parts[len(parts)-1])
	if p, err := strconv.Atoi(last); err == nil && p > 0 {
		return p
	}
	return 10000
}

func startWorkerWithRetry(pluginFile string, nReducer int, startPort int, step int, storeInRAM bool) {
	if err := startWorkerWithRetryE(pluginFile, nReducer, startPort, step, storeInRAM); err != nil {
		panic(err)
	}
}

func startWorkerWithRetryE(pluginFile string, nReducer int, startPort int, step int, storeInRAM bool) error {
	const maxAttempts = 128
	if step <= 0 {
		step = 1
	}
	for i := 0; i < maxAttempts; i++ {
		port := startPort + i*step
		addr := fmt.Sprintf(":%d", port)
		if err := startWorkerOnce(pluginFile, nReducer, addr, storeInRAM); err != nil {
			msg := err.Error()
			if strings.Contains(msg, "address already in use") {
				fmt.Printf("worker listen %s occupied, trying next port\n", addr)
				continue
			}
			return err
		}
		return nil
	}
	return fmt.Errorf("unable to find available worker port from %d after %d attempts", startPort, maxAttempts)
}

func startWorkerOnce(pluginFile string, nReducer int, addr string, storeInRAM bool) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	worker.StartWorker(pluginFile, nReducer, addr, storeInRAM)
	return nil
}
