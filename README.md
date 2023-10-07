<h1 align="center">tvbase</h1>
<p align="center">TVN node for message protocol and DKVS</p>

## Getting started for tvbase

Compile program need install golang

```shell
### prepare init/run ipfs if need
## ipfs init
## ipfs daemon --enable-gc
## ipfs shutdown

### start tvnode
git clone git@github.com:tinyverse-web3/tvbase.git
cd tvbase/tvnode
go build
# -init option will generate identity.bin and default config.json, -help optin if necessary more help
./tvnode -init
./boot.sh


## start tvnode light
cd ../tvnodelight
go build
# the cmd option -src represents the user ID of the sender, 
# the cmd option -dest represents the receiving end user ID
# thd cmd option -path represents root path for identify.bin, config.json and data files
# run tvnodelight1 in a terminal
./tvnodelight -src a -dest b -path ./a
# run tvnodelight2 in other terminal
./tvnodelight -src b -dest a -path ./b
# finally, enter content through the terminal at the two terminals for message communication
```

## develope
```shell
#  go get private github need to execute the following script
go env -w GOPRIVATE="github.com/tinyverse-web3/*"

echo '[url "git@github.com:tinyverse-web3/"]
	insteadOf = https://github.com/tinyverse-web3/' >> ~/.gitconfig
```

## deploy full tvnode in linux server cmd
# clear all data and deploy
```shell
cd ~/tvbase/tvnode && git pull && chmod +x ./install.sh  && ./install.sh && rm ~/.tvnode/*.log && rm -rf ~/.tvnode/*_data && rm -rf ~/.tvnode/unsynckv && ./boot.sh
```

# Just deploy without clearing any data
```shell
cd ~/tvbase/tvnode && git pull && chmod +x ./install.sh  && ./install.sh && ./boot.sh
```

## config github private repository, need join tinyverse-web3 org
```shell
echo '[url "git@github.com:tinyverse-web3/"]
 insteadOf = https://github.com/tinyverse-web3/'  >> ~/.gitconfig 

go env -w GOPRIVATE="github.com/tinyverse-web3/*"
```

## compile sdk
```shell
gomobile bind -o ./output/android/core.aar -target=android -androidapi 19 github.com/tinyverse-web3/gomobile-libp2p/go/bind/core 

```
