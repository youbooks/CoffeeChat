package tcpserver

import (
	"context"
	"fmt"
	"github.com/CoffeeChat/server/src/api/cim"
	"github.com/CoffeeChat/server/src/internal/gate/conf"
	"github.com/CoffeeChat/server/src/pkg/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"time"
)

// all grpc client connections
var logicClientMap map[int]*LogicGrpcClient = nil

const kMaxLogicClientConnect = 0
const kMaxCheckInterval = 32 // s

type LogicGrpcClient struct {
	instance    cim.LogicClient
	conn        *grpc.ClientConn
	config      conf.LogicConfig
	index       int
	isConnected bool
}

func init() {
	logicClientMap = make(map[int]*LogicGrpcClient)
	c := time.Tick(time.Duration(time.Second * 1))

	go func() {
		var checkTime int64 = 1
		var tick int64 = 0
		for {
			<-c

			// 1s 2s 4s 8s 16s 32s
			if tick%checkTime == 0 {
				var isConnected = true
				for i := range logicClientMap {
					if logicClientMap[i].conn.GetState() != connectivity.Ready {
						logger.Sugar.Errorf("grpc server not connected,%s:%d,index=%d",
							logicClientMap[i].config.Ip, logicClientMap[i].config.Port, logicClientMap[i].index)
						isConnected = false
						logicClientMap[i].isConnected = false
					} else {
						if !logicClientMap[i].isConnected {
							// gRPC sayHello
							for j := 1; j <= 3; j++ {
								err := sayHello(logicClientMap[i].instance)
								if err != nil {
									logger.Sugar.Infof("grpc server connected success,but sayHello failed,retry=%d,%s:%d,index=%d,err=%s",
										j, logicClientMap[i].config.Ip, logicClientMap[i].config.Port, logicClientMap[i].index, err.Error())
									time.Sleep(10 * time.Millisecond)
								} else {
									logger.Sugar.Infof("grpc server connected success,%s:%d,index=%d",
										logicClientMap[i].config.Ip, logicClientMap[i].config.Port, logicClientMap[i].index)
									logicClientMap[i].isConnected = true
									break
								}
							}
						}
					}
				}
				if checkTime > kMaxCheckInterval {
					checkTime = 1
				}
				if !isConnected {
					checkTime *= 2
				}
			}

			tick++
		}
	}()
}

func StartGrpcClient(config []conf.LogicConfig) {
	logger.Sugar.Info("start grpc client")

	var curCount = 0
	for i := range config {
		logger.Sugar.Infof("logic rpc server ip=%s,port=%d,maxConnCnt=%d", config[i].Ip, config[i].Port, config[i].MaxConnCnt)

		address := fmt.Sprintf("%s:%d", config[i].Ip, config[i].Port)
		//maxConnCnt := config[i].MaxConnCnt

		conn, err := grpc.Dial(address, grpc.WithInsecure())
		if err != nil {
			logger.Sugar.Error("connect grpc server=%s,error:", address, err.Error())
		} else {
			client := cim.NewLogicClient(conn)
			// save
			logicClientMap[curCount] = &LogicGrpcClient{
				instance:    client,
				conn:        conn,
				config:      config[i],
				index:       i,
				isConnected: true,
			}
			// sayHello
			err := sayHello(client)
			if err != nil {
				// if failed, the routine will try again
				logicClientMap[curCount].isConnected = false
			}
		}
		curCount++
	}
}

func sayHello(conn cim.LogicClient) error {
	heart := &cim.Hello{
		Ip:   conf.DefaultConfig.ListenIpGrpc,
		Port: int32(conf.DefaultConfig.ListenPortGrpc),
	}
	_, err := conn.SayHello(context.Background(), heart)
	return err
}

// 获取登录验证的业务连接
func GetLoginConn() cim.LogicClient {
	return logicClientMap[0].instance
}
