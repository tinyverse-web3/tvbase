package tvbase

import (
	"os"
	"runtime/pprof"
	"time"

	tvLog "github.com/tinyverse-web3/tvbase/common/log"
)

const (
	EnvEnableProfiling = "tvbase_PROF"
	cpuProfile         = "tvbase.cpuprof"
	heapProfile        = "tvbase.memprof"
)

func profileIfEnabled() (func(), error) {
	// FIXME this is a temporary hack so profiling of asynchronous operations
	// works as intended.
	stopProfilingFunc, err := startProfiling() // TODO maybe change this to its own option... profiling makes it slower.
	if err != nil {
		return nil, err
	}
	return stopProfilingFunc, nil
}

// startProfiling begins CPU profiling and returns a `stop` function to be
// executed as late as possible. The stop function captures the memprofile.
func startProfiling() (func(), error) {
	// start CPU profiling as early as possible
	ofi, err := os.Create(cpuProfile)
	if err != nil {
		return nil, err
	}
	err = pprof.StartCPUProfile(ofi)
	if err != nil {
		ofi.Close()
		return nil, err
	}
	go func() {
		for range time.NewTicker(time.Second * 30).C {
			err := writeHeapProfileToFile()
			if err != nil {
				tvLog.Logger.Error(err)
			}
		}
	}()

	stopProfiling := func() {
		pprof.StopCPUProfile()
		ofi.Close() // captured by the closure
	}
	return stopProfiling, nil
}

func writeHeapProfileToFile() error {
	mprof, err := os.Create(heapProfile)
	if err != nil {
		return err
	}
	defer mprof.Close() // _after_ writing the heap profile
	return pprof.WriteHeapProfile(mprof)
}
