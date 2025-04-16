package auth

import (
	"context"
	"errors"
	"strings"

	"go-micro.dev/v4"
	"go-micro.dev/v4/auth"
	"go-micro.dev/v4/metadata"
	"go-micro.dev/v4/server"
)

// 用于确保只有合法worker节点可以调用master的grpc接口，防止未授权节点获取任务分配或篡改资源

// 创建认证中间件包装器
func NewAuthWrapper(service micro.Service) server.HandlerWrapper {
	// 返回中间件包装函数
	return func(h server.HandlerFunc) server.HandlerFunc {
		// 实际包装后的处理函数
		return func(ctx context.Context, req server.Request, rsp interface{}) error {
			// Fetch metadata from context (request headers).
			// 从上下文获取元数据，即http请求头
			md, b := metadata.FromContext(ctx)
			if !b {
				return errors.New("no metadata found")
			}

			// Get auth header.
			// 获取Authorization请求头
			authHeader, ok := md["Authorization"]
			// 验证Authorization头格式，必须以Bearer开头
			if !ok || !strings.HasPrefix(authHeader, auth.BearerScheme) {
				return errors.New("no auth token provided")
			}

			// Extract auth token.
			// 提取真正的token，即去除Bearer前缀
			token := strings.TrimPrefix(authHeader, auth.BearerScheme)

			// Extract account from token.
			// 从服务配置获取认证组件
			a := service.Options().Auth
			// 验证token是否有效，Inspect方法会校验签名/有效期等
			_, err := a.Inspect(token)
			if err != nil {
				return errors.New("auth token invalid")
			}

			// 认证通过，继续执行原始处理逻辑
			return h(ctx, req, rsp)
		}
	}
}
