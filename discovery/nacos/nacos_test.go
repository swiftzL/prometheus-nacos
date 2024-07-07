package nacos

import (
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/common/logger"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"go.uber.org/zap"
	"testing"
)

func TestRegister(t *testing.T) {
	//192.168.31.157:9100

	zapLoggerConfig := zap.NewProductionConfig()
	zapLogger, _ := zapLoggerConfig.Build(zap.AddCaller(), zap.AddCallerSkip(1))
	logger.SetLogger(&logger.NacosLogger{zapLogger.Sugar()})

	serverConfigs := []constant.ServerConfig{
		*constant.NewServerConfig(
			"192.168.31.157",
			8848,
			constant.WithScheme("http"),
			constant.WithContextPath("/nacos"),
		),
	}
	namingClient, err := clients.NewNamingClient(
		vo.NacosClientParam{
			ClientConfig: &constant.ClientConfig{
				TimeoutMs:    10 * 1000,
				BeatInterval: 5 * 1000,
				LogLevel:     "INFO",
			},
			ServerConfigs: serverConfigs,
		},
	)
	if err != nil {
		panic(err)
	}
	_, err = namingClient.RegisterInstance(vo.RegisterInstanceParam{
		Ip:          "192.168.31.157",
		Port:        9100,
		Weight:      1,
		Enable:      true,
		ClusterName: "",
		ServiceName: "game",
		GroupName:   "",
		Ephemeral:   false,
	})
	if err != nil {
		panic(err)
	}
	select {}
}
