package ipfs

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	ipfsLog "github.com/ipfs/go-log/v2"
)

const LoggerName = "tvipfs"

var Logger = ipfsLog.Logger(LoggerName)

const (
	ObjectStatusField_NumLinks       = "NumLinks:"
	ObjectStatusField_BlockSize      = "BlockSize:"
	ObjectStatusField_LinksSize      = "LinksSize:"
	ObjectStatusField_DataSize       = "DataSize:"
	ObjectStatusField_CumulativeSize = "CumulativeSize:"
)

type PidStatus int

const (
	PinStatus_UNKNOW PidStatus = iota
	PinStatus_INIT
	PinStatus_ERR
	PinStatus_WORK
	PinStatus_PINNED
	PinStatus_ALREADY_PINNED
	PinStatus_TIMEOUT
)

type CidObjectLink struct {
	Cid  string
	Size int64
}

func CheckIpfsCmd() error {
	out, err := exec.Command("ipfs", "version").CombinedOutput()
	if err != nil {
		Logger.Errorf("CheckIpfsCmd err: %v, out: %s", err, out)
		return err
	}
	Logger.Debugf("CheckIpfsCmd: out: %s", out)
	return nil
}

func IpfsBlockGet(cid string, ctx context.Context) ([]byte, time.Duration, error) {
	startTime := time.Now()
	cmdOut, err := exec.CommandContext(ctx, "ipfs", "block", "get", cid).CombinedOutput()
	elapsed := time.Since(startTime)
	Logger.Debugf("IpfsBlockGet:\ncid: %s,\ncmd out: %s,\nelapsed time: %v", cid, cmdOut, elapsed.Seconds())
	if err != nil {
		Logger.Errorf("IpfsBlockGet:\ncid: %s,\ncmd out: %s,\nelapsed time: %v, err: %v", cid, cmdOut, elapsed.Seconds(), err)
		return cmdOut, elapsed, err
	}
	return cmdOut, elapsed, nil
}

func IpfsGet(cid string, ctx context.Context) (time.Duration, error) {
	startTime := time.Now()
	cmdOut, err := exec.CommandContext(ctx, "ipfs", "get", cid).CombinedOutput()
	elapsed := time.Since(startTime)
	Logger.Debugf("IpfsGet: cid: %s, cmd out: %s, elapsed time: %v", cid, cmdOut, elapsed.Seconds())
	if err != nil {
		Logger.Errorf("IpfsGet: cid: %s, cmd out: %s, elapsed time: %v, err: %v", cid, cmdOut, elapsed.Seconds(), err)
		return elapsed, err
	}
	return elapsed, nil
}

func IpfsPinLs(cid string, ctx context.Context) (bool, error) {
	cmdOut, err := exec.CommandContext(ctx, "ipfs", "pin", "ls", "--type", "recursive", cid).CombinedOutput()
	Logger.Debugf("IpfsPinLs: cid: %s, cmd out: %s", cid, cmdOut)
	if err != nil {
		if strings.Contains(string(cmdOut), "is not pinned") {
			return false, nil
		}
		Logger.Errorf("IpfsPinLs: cid: %s, cmd out: %s, err: %v", cid, cmdOut, err)
		return false, err
	}
	return true, nil
}

func IpfsPinAdd(cid string, ctx context.Context) (time.Duration, error) {
	startTime := time.Now()
	cmdOut, err := exec.CommandContext(ctx, "ipfs", "pin", "add", cid).CombinedOutput()
	elapsed := time.Since(startTime)
	Logger.Debugf("IpfsPinAdd: cid: %s, cmd out: %s, elapsed time: %v", cid, cmdOut, elapsed.Seconds())
	if err != nil {
		Logger.Errorf("IpfsPinAdd: cid: %s, cmd out: %s, elapsed time: %v, err: %v", cid, cmdOut, elapsed.Seconds(), err)
		return elapsed, err
	}
	return elapsed, nil
}

func IpfsPinRm(cid string, ctx context.Context) error {
	cmdOut, err := exec.CommandContext(ctx, "ipfs", "pin", "rm", cid).CombinedOutput()
	Logger.Debugf("IpfsPinRm: cid: %s, cmd out: %s", cid, cmdOut)
	if err != nil {
		Logger.Errorf("IpfsPinRm: cid: %s, cmd out: %s, err: %v", cid, cmdOut, err)
		return err
	}
	return nil
}

func IpfsPinUpdate(fromCid string, toCid string, ctx context.Context) (time.Duration, error) {
	// must call IpfsPinLs(toCid, ctx) first
	startTime := time.Now()
	cmdOut, err := exec.CommandContext(ctx, "ipfs", "pin", "update", fromCid, toCid).CombinedOutput()
	elapsed := time.Since(startTime)
	Logger.Debugf("IpfsPinUpdate: cid: %s, cmd out: %s, elapsed time: %v", fromCid, cmdOut, elapsed.Seconds())
	if err != nil {
		Logger.Errorf("IpfsPinUpdate: cid: %s, cmd out: %s, elapsed time: %v, err: %v", fromCid, cmdOut, elapsed.Seconds(), err)
		return elapsed, err
	}
	return elapsed, nil
}

func IpfsObjectStat(cid string, ctx context.Context) (*map[string]int64, time.Duration, error) {
	startTime := time.Now()
	cmdOut, err := exec.CommandContext(ctx, "ipfs", "object", "stat", cid).CombinedOutput()
	elapsed := time.Since(startTime)
	Logger.Debugf("IpfsObjectStat: cid: %s, cmd out: %s, elapsed time: %v", cid, cmdOut, elapsed.Seconds())
	if err != nil {
		Logger.Errorf("IpfsObjectStat: cid: %s, cmd out: %s, elapsed time: %v, err: %v", cid, cmdOut, elapsed.Seconds(), err)
		return nil, elapsed, err
	}

	lines := strings.Split(string(cmdOut), "\n")
	if len(lines) < 5 {
		Logger.Errorf("IpfsObjectStat: cid:%s, cmd out: %s, elapsed time: %v, err: %v", cid, cmdOut, elapsed.Seconds(), "lines != 5")
		return nil, elapsed, fmt.Errorf("IpfsObjectStat: cid:%s, cmd out: %s, elapsed time: %v, err: %v", cid, cmdOut, elapsed.Seconds(), "lines != 5")
	}
	cidStatInfo := make(map[string]int64)

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		size, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			Logger.Errorf("IpfsObjectStat: cid:%s, cmd out: %s, elapsed time: %v, err: %v", cid, cmdOut, elapsed.Seconds(), "error parsing size")
			continue
		}
		key := fields[0]
		cidStatInfo[key] = size
	}
	_, ok := cidStatInfo[ObjectStatusField_NumLinks]
	if !ok {
		Logger.Errorf("IpfsObjectStat: cid:%s, cmd out: %s, elapsed time: %v, err: %v", cid, cmdOut, elapsed.Seconds(), "key not found")
		return nil, elapsed, err
	}
	_, ok = cidStatInfo[ObjectStatusField_BlockSize]
	if !ok {
		Logger.Errorf("IpfsObjectStat: cid:%s, cmd out: %s, elapsed time: %v, err: %v", cid, cmdOut, elapsed.Seconds(), "key not found")
		return nil, elapsed, err
	}
	_, ok = cidStatInfo[ObjectStatusField_LinksSize]
	if !ok {
		Logger.Errorf("IpfsObjectStat: cid:%s, cmd out: %s, elapsed time: %v, err: %v", cid, cmdOut, elapsed.Seconds(), "key not found")
		return nil, elapsed, err
	}
	_, ok = cidStatInfo[ObjectStatusField_DataSize]
	if !ok {
		Logger.Errorf("IpfsObjectStat: cid:%s, cmd out: %s, elapsed time: %v, err: %v", cid, cmdOut, elapsed.Seconds(), "key not found")
		return nil, elapsed, err
	}
	_, ok = cidStatInfo[ObjectStatusField_CumulativeSize]
	if !ok {
		Logger.Errorf("IpfsObjectStat: cid:%s, cmd out: %s, elapsed time: %v, err: %v", cid, cmdOut, elapsed.Seconds(), "key not found")
		return nil, elapsed, err
	}
	return &cidStatInfo, elapsed, nil
}

func IpfsGetObject(cid string, ctx context.Context, checkTimeout time.Duration) (int64, time.Duration, PidStatus, error) {
	isAlreadyPin, err := IpfsPinLs(cid, ctx)
	if err != nil {
		Logger.Errorf("IpfsGetObject: err: %v", err)
		return 0, 0, PinStatus_ERR, err
	}

	cumulativeSize := int64(0)
	allElapsedTime := time.Duration(0)
	pinStatus := PinStatus_INIT
	var lastErr error

	if isAlreadyPin {
		cidStat, elapsedTime, err := IpfsObjectStat(cid, ctx)
		allElapsedTime += elapsedTime
		lastErr = err
		if err != nil {
			Logger.Errorf("IpfsGetObject: err: %v", err)
			return 0, allElapsedTime, pinStatus, lastErr
		}
		cumulativeSize = (*cidStat)[ObjectStatusField_CumulativeSize]
		pinStatus = PinStatus_ALREADY_PINNED
		Logger.Debugf("IpfsGetObject: cid: %s, allElapsedTime: %v, pinStatus: %v, lastErr: %v", cid, allElapsedTime, pinStatus, lastErr)
		return cumulativeSize, allElapsedTime, pinStatus, lastErr
	}

	pinStatus = PinStatus_WORK
	if checkTimeout <= 0 {
		checkTimeout = 3 * time.Minute
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	cidStat, elapsedTime, err := IpfsObjectStat(cid, timeoutCtx)
	allElapsedTime += elapsedTime
	if err != nil {
		Logger.Errorf("IpfsGetObject: err: %v", err)
		lastErr = err
		pinStatus = PinStatus_ERR
		if strings.Contains(err.Error(), "context deadline exceeded") {
			pinStatus = PinStatus_TIMEOUT
			Logger.Debugf("IpfsGetObject: cid: %s, allElapsedTime: %v, pinStatus: %v, lastErr: %v", cid, allElapsedTime, pinStatus, lastErr)
		} else {
			Logger.Errorf("IpfsGetObject: cid: %s, allElapsedTime: %v, pinStatus: %v, lastErr: %v", cid, allElapsedTime, pinStatus, lastErr)
		}
		return 0, allElapsedTime, pinStatus, lastErr
	}

	cumulativeSize = (*cidStat)[ObjectStatusField_CumulativeSize]

	if (*cidStat)[ObjectStatusField_NumLinks] == 0 {
		elapsedTime, err := IpfsPinAdd(cid, timeoutCtx)
		allElapsedTime += elapsedTime
		if err != nil {
			Logger.Errorf("IpfsGetObject: elasped time: %v, err: %v", allElapsedTime, err)
			lastErr = err
			pinStatus = PinStatus_ERR
			if strings.Contains(err.Error(), "context deadline exceeded") {
				pinStatus = PinStatus_TIMEOUT
				Logger.Debugf("IpfsGetObject: cid: %s, allElapsedTime: %v, pinStatus: %v, lastErr: %v", cid, allElapsedTime, pinStatus, lastErr)
			} else {
				Logger.Errorf("IpfsGetObject: cid: %s, allElapsedTime: %v, pinStatus: %v, lastErr: %v", cid, allElapsedTime, pinStatus, lastErr)
			}
			return cumulativeSize, allElapsedTime, pinStatus, lastErr
		}

		pinStatus = PinStatus_PINNED
		Logger.Debugf("IpfsGetObject: cid: %s, allElapsedTime: %v, pinStatus: %v, lastErr: %v", cid, allElapsedTime, pinStatus, lastErr)
		return cumulativeSize, allElapsedTime, pinStatus, lastErr
	}

	objectLinks, elapsedTime, err := IpfsGetObjectLinks(cid, timeoutCtx)
	allElapsedTime += elapsedTime
	if err != nil {
		Logger.Errorf("IpfsGetObject: err: %v", err)
		lastErr = err
		pinStatus = PinStatus_ERR
		if strings.Contains(err.Error(), "context deadline exceeded") {
			pinStatus = PinStatus_TIMEOUT
			Logger.Debugf("IpfsGetObject: cid: %s, allElapsedTime: %v, pinStatus: %v, lastErr: %v", cid, allElapsedTime, pinStatus, lastErr)
		} else {
			Logger.Errorf("IpfsGetObject: cid: %s, allElapsedTime: %v, pinStatus: %v, lastErr: %v", cid, allElapsedTime, pinStatus, lastErr)
		}
		return cumulativeSize, allElapsedTime, pinStatus, lastErr
	}

	link := CidObjectLink{
		Cid:  cid,
		Size: cumulativeSize,
	}
	objectLinks = append(objectLinks, link)

	var pinCtx context.Context
	var pinCancel context.CancelFunc
	estimatedTime := 1 * time.Second * time.Duration(len(objectLinks)) * 10
	if estimatedTime > 0 {
		pinCtx, pinCancel = context.WithTimeout(ctx, estimatedTime)
		defer pinCancel()
	}
	for _, link := range objectLinks {
		elapsedTime, err = IpfsPinAdd(link.Cid, pinCtx)
		allElapsedTime += elapsedTime
		if err != nil {
			Logger.Errorf("IpfsGetObject: elasped time: %v, err: %v", allElapsedTime, err)
			lastErr = err
			pinStatus = PinStatus_ERR
			if strings.Contains(err.Error(), "context deadline exceeded") {
				pinStatus = PinStatus_TIMEOUT
				Logger.Debugf("IpfsGetObject: cid: %s, allElapsedTime: %v, pinStatus: %v, lastErr: %v", cid, allElapsedTime, pinStatus, lastErr)
			} else {
				Logger.Errorf("IpfsGetObject: cid: %s, allElapsedTime: %v, pinStatus: %v, lastErr: %v", cid, allElapsedTime, pinStatus, lastErr)
			}
			return cumulativeSize, allElapsedTime, pinStatus, lastErr
		}
	}
	pinStatus = PinStatus_PINNED
	Logger.Debugf("IpfsGetObject: cid: %s, allElapsedTime: %v, pinStatus: %v, lastErr: %v", cid, allElapsedTime, pinStatus, lastErr)
	return cumulativeSize, allElapsedTime, pinStatus, lastErr
}

func IpfsGetObjectLinks(cid string, ctx context.Context) ([]CidObjectLink, time.Duration, error) {
	startTime := time.Now()
	cmdOut, err := exec.CommandContext(ctx, "ipfs", "object", "links", cid).CombinedOutput()
	allElapsedTime := time.Since(startTime)

	if err != nil {
		Logger.Errorf("IpfsGetOjbectLinks: cid: %s, cmd out: %s, elapsed time: %v, err: %v", cid, cmdOut, allElapsedTime.Seconds(), err)
		return nil, allElapsedTime, err
	}

	objectLinks := make([]CidObjectLink, 0)
	lines := strings.Split(string(cmdOut), "\n")

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		childCid := fields[0]
		cidDataSize, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			continue
		}

		childObjectlinks, childElapsedTime, err := IpfsGetObjectLinks(childCid, ctx)
		allElapsedTime += childElapsedTime
		if err != nil {
			Logger.Errorf("IpfsGetOjbectLinks: cid: %s, cmd out: %s, elapsed time: %v, err: %v", childCid, cmdOut, allElapsedTime.Seconds(), err)
			return nil, allElapsedTime, err
		}
		link := CidObjectLink{
			Cid:  childCid,
			Size: cidDataSize,
		}
		if len(childObjectlinks) == 0 {
			objectLinks = append(objectLinks, link)
		} else {
			objectLinks = append(objectLinks, childObjectlinks...)
			objectLinks = append(objectLinks, link)
		}
	}
	Logger.Debugf("IpfsGetOjbectLinks: cmd out: %s, elapsed time: %v", cmdOut, allElapsedTime.Seconds())
	return objectLinks, allElapsedTime, nil
}

func RoutingFindProvs(
	ctx context.Context,
	cid string,
	maxProviders int,
) (elapsedTime time.Duration, providerList []string, err error) {
	startTime := time.Now()
	cmdOut, err := exec.CommandContext(ctx, "ipfs", "routing", "findprovs", cid, fmt.Sprintf("--num-providers=%d", maxProviders)).CombinedOutput()
	elapsedTime = time.Since(startTime)
	Logger.Debugf("RoutingFindProvs: \ncid: %s\nmaxProviders:%d\ncmdout:\n%s\nelapsed time: %+v",
		cid, maxProviders, cmdOut, elapsedTime.Seconds())
	if err != nil {
		Logger.Errorf("RoutingFindProvs: \ncid: %s\nmaxProviders:%d\ncmdout:\n%s\nelapsed time: %+v, error:%+v",
			cid, maxProviders, cmdOut, elapsedTime.Seconds(), err)
		return elapsedTime, providerList, err
	}
	providerList = strings.Split(strings.TrimRight(string(cmdOut), "\n"), "\n")
	return elapsedTime, providerList, nil
}
