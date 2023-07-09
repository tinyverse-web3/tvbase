package test

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"testing"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	tvConfig "github.com/tinyverse-web3/tvbase/common/config"
	tvLog "github.com/tinyverse-web3/tvbase/common/log"
	tvUtil "github.com/tinyverse-web3/tvbase/common/util"
	"github.com/tinyverse-web3/tvbase/dmsg/protocol/custom/pullcid"
	dmsgService "github.com/tinyverse-web3/tvbase/dmsg/service"
	"github.com/tinyverse-web3/tvbase/dmsg/service/common"
	"github.com/tinyverse-web3/tvbase/tvbase"
)

// msg service
func TestMsgService(t *testing.T) {
	// t.Parallel()
	// t.Skip()
	rootPath := parseServiceCmdParams()

	nodeConfig, err := tvUtil.LoadNodeConfig(rootPath)
	if err != nil {
		tvLog.Logger.Errorf("TestMsgService error: %v", err)
		t.Errorf("TestMsgService error: %v", err)
		return
	}
	err = tvUtil.SetLogModule(nodeConfig.Log.ModuleLevels)
	if err != nil {
		tvLog.Logger.Errorf("TestMsgService error: %v", err)
		t.Errorf("TestMsgService error: %v", err)
		return
	}

	ctx := context.Background()
	tvbase, err := tvbase.NewTvbase(rootPath, ctx, true)
	if err != nil {
		panic(err)
	}
	tvbase.RegistCSSProtocol(pullcid.GetPullCidServiceProtocol())

	defer func() {
		tvbase.Stop()
		select {
		case <-ctx.Done():
			tvLog.Logger.Info("Gracefully shut down daemon")
		default:
		}
	}()

	go readConsoleToSendMsg(tvbase)
	<-ctx.Done()
}

func parseServiceCmdParams() string {
	generateCfg := flag.Bool("init", false, "init generate identityKey and config file")
	rootPath := flag.String("rootPath", "", "config file path")
	help := flag.Bool("help", false, "Display help")

	flag.Parse()

	if *help {
		tvLog.Logger.Info("tinyverse tnnode\n")
		tvLog.Logger.Info("Usage step1: Run './tvnode -init' generate identityKey and config.")
		tvLog.Logger.Info("Usage step2: Run './tvnode' or './tvnode -rootPath .' start tvnode service.")
		os.Exit(0)
	}
	if *generateCfg {
		err := tvUtil.GenConfig2IdentityFile(*rootPath, tvConfig.FullMode)
		if err != nil {
			tvLog.Logger.Fatal(err)
		}
		os.Exit(0)
	}
	return *rootPath
}

func readConsoleToSendMsg(node *tvbase.TvBase) {
	dmsgService := node.DmsgService.(*dmsgService.DmsgService)

	pk := "0400d3192b5e36d458bce6b8b7c9fbe19c90acfd01a6da7f01cf4729ac3976c957c2ac4ab38ff899fcdca6ddba661785c34eb00c2cd5b2b6d014ca6911463b3fa2"
	var destPubsub *common.DestUserPubsub

	// wait tvnodelight connect
	for {
		pubsub := dmsgService.GetDestUserPubsub(pk)
		if pubsub != nil {
			destPubsub = pubsub
			break
		}
		time.Sleep(10 * time.Second)
	}
	ctx := context.Background()

	go streamConsoleTo(ctx, destPubsub.UserTopic)

	go func() {
		m, err := destPubsub.UserSub.Next(ctx)
		if err != nil {
			panic(err)
		}
		fmt.Println(m.ReceivedFrom, ": ", string(m.Message.Data))
	}()

	err := dmsgService.PublishProtocol(3, pk, []byte("hello"))
	if err != nil {
		panic(err)
	}
}

func streamConsoleTo(ctx context.Context, topic *pubsub.Topic) {
	reader := bufio.NewReader(os.Stdin)
	for {
		s, err := reader.ReadString('\n')
		if err != nil {
			panic(err)
		}
		if err := topic.Publish(ctx, []byte(s)); err != nil {
			fmt.Println("### Publish error:", err)
		}
	}
}
