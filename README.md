# fabric-CoalMine
Coal Mine Scheduling Speech Storage Based on Consortium Blockchain and Speech Macromodelling
## 一、启动区块链网络
1.首先进入用于启动网络的脚本所在的目录：  
```
cd fabric-samples/test-network
```

2.删除先前运行的任何容器或区块链网络：
```
./network.sh down
```
3.然后用以下命令来启动网络并创建通道mychannel，此命令创建一个由两个对等组织和一个 ordering 节点组成的 Fabric 网络：  
```
./network.sh up createChannel
```
4.部署链码（即智能合约）
```
./network.sh deployCC -ccn basic -ccp ../asset-transfer-basic/chaincode-go -ccl go
```

如需添加新的组织到通道中，请参考: [添加组织3到频道并部署链码](#添加组织3到频道并部署链码)

## 二、测试
1.首先进入测试工具Caliper CLI所在的目录：  
```
cd hyperledger/caliper/workspace
```

2.通过以下命令，利用 NPM 软件包安装 Caliper CLI。
根据"config.yaml,test-network.yaml"和  
"./hyperledger/caliper/workspace/benchmarks/samples/fabric/basic/"下的各种待测试的合约函数配置文件进行基准测试。
```
npm install --only=prod @hyperledger/caliper-cli@0.6.0
npx caliper bind --caliper-bind-sut fabric:2.5
npx caliper launch manager --caliper-workspace ./ --caliper-networkconfig networks/fabric/test-network.yaml \
--caliper-benchconfig benchmarks/samples/fabric/basic/config.yaml --caliper-flow-only-test --caliper-fabric-gateway-enabled
```


## 添加组织3到频道并部署链码
创建组织3。在fabric-samples/test-network下执行：
```
cd addOrg3
./addOrg3.sh up -c mychannel
```
Org1 和 Org2 节点上安装完链码后，使用以下环境变量，以便作为 Org3 与区块链网络进行交互管理  
```
export PATH=${PWD}/../bin:$PATH
export FABRIC_CFG_PATH=$PWD/../config/
export CORE_PEER_TLS_ENABLED=true
export CORE_PEER_LOCALMSPID=Org3MSP
export CORE_PEER_TLS_ROOTCERT_FILE=${PWD}/organizations/peerOrganizations/org3.mine.com/peers/peer0.org3.mine.com/tls/ca.crt
export CORE_PEER_MSPCONFIGPATH=${PWD}/organizations/peerOrganizations/org3.mine.com/users/Admin@org3.mine.com/msp
export CORE_PEER_ADDRESS=localhost:11051
```
使用'peer lifecycle chaincode queryinstalled'命令查询Org3的Peer节点，得到形如下面的软件包的信息：  
```
Installed chaincodes on peer:
Package ID: basic_1.0.1:b5464a7ae883fec1b4a16ade22166233967c6f0feae1545069f7020397c3cf7a, Label: basic_1.0.1
```
将此软件包 ID，另存为环境变量。注意'export CC_PACKAGE_ID='后面衔接的是之上的"Package ID"。
```
export CC_PACKAGE_ID=basic_1.0.1:b5464a7ae883fec1b4a16ade22166233967c6f0feae1545069f7020397c3cf7a
```
使用以下命令审批组织3的基本链码定义，并检查该链码定义是否已经提交给通道mychanel：
```
peer lifecycle chaincode approveformyorg -o localhost:7050 --ordererTLSHostnameOverride orderer.mine.com --tls --cafile "${PWD}/organizations/ordererOrganizations/mine.com/orderers/orderer.mine.com/msp/tlscacerts/tlsca.mine.com-cert.pem" --channelID mychannel --name basic --version 1.0.1 --package-id $CC_PACKAGE_ID --sequence 1

peer lifecycle chaincode querycommitted --channelID mychannel --name basic
```
至此，新的组织创建加入到通道mychanel，并部署了其上的链码。


