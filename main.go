package main

import (
	"github.com/alibaba/higress/plugins/wasm-go/pkg/log"
	"github.com/alibaba/higress/plugins/wasm-go/pkg/wrapper"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/tidwall/gjson"
	"github.com/tidwall/resp"
)

func main() {
	wrapper.SetCtx(
		"redis-demo",
		wrapper.ParseConfig(parseConfig),
		wrapper.ProcessRequestHeaders(onHttpRequestHeaders),
		wrapper.ProcessResponseHeaders(onHttpResponseHeaders),
	)
}

type RedisCallConfig struct {
	client wrapper.RedisClient
	qpm    int
}

func parseConfig(json gjson.Result, config *RedisCallConfig) error {
	log.Infof("parseConfig:%s", json.String())
	redisName := json.Get("redisName").String()
	redisPort := json.Get("redisPort").Int()
	username := json.Get("username").String()
	password := json.Get("password").String()
	timeout := json.Get("timeout").Int()
	qpm := json.Get("qpm").Int()
	config.qpm = int(qpm)
	config.client = wrapper.NewRedisClusterClient(wrapper.FQDNCluster{
		FQDN: redisName,
		Port: redisPort,
	})
	return config.client.Init(username, password, timeout)
}

func onHttpRequestHeaders(ctx wrapper.HttpContext, config RedisCallConfig) types.Action {
	sid, _ := proxywasm.GetHttpRequestHeader("x-sid")
	log.Infof("x-sid: %s", sid)
	config.client.SetEx("higress:go:wasm:test", sid, 1000000, func(response resp.Value) {
		if err := response.Error(); err != nil {
			log.Errorf("set redis:%s", err.Error())
			proxywasm.SendHttpResponse(430, nil, []byte("Error while calling redis"), -1)
		} else {
			log.Infof("value:%s", response.String())
		}
	})
	// 通过redis获取token
	err := config.client.Get("sei:auth:login:"+sid, func(response resp.Value) {
		if err := response.Error(); err != nil {
			log.Errorf("get oa:%s", err.Error())
			proxywasm.SendHttpResponse(430, nil, []byte("Error while calling redis"), -1)
		} else {
			log.Infof("value")
			err := proxywasm.AddHttpResponseHeader("x-authorization", response.String())
			if err != nil {
				return
			} else {
				log.Errorf("Error occured while calling redis:%s", err.Error())
			}
		}
	})
	if err != nil {
		// 由于调用redis失败，放行请求，记录日志
		log.Errorf("Error occured while calling redis, it seems cannot find the redis cluster.")
		return types.ActionContinue
	} else {
		// 请求hold住，等待redis调用完成
		log.Infof("wating")
		return types.ActionPause
	}
}

func onHttpResponseHeaders(ctx wrapper.HttpContext, config RedisCallConfig) types.Action {
	log.Infof("onHttpResponseHeaders")
	return types.ActionContinue
}
