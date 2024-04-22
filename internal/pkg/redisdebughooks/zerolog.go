package redisdebughooks

import (
	"context"
	"net"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

type ZerologRedisHook struct {
	logger *zerolog.Logger
}

func NewZerologRedisHook(logger *zerolog.Logger) *ZerologRedisHook {
	return &ZerologRedisHook{
		logger: logger,
	}
}

// DialHook implements redis.Hook.
func (z *ZerologRedisHook) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		return next(ctx, network, addr)
	}
}

// ProcessHook implements redis.Hook.
func (z *ZerologRedisHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		z.logger.Debug().Any("cmd", cmd).Msg("process hook")

		return next(ctx, cmd)
	}
}

// ProcessPipelineHook implements redis.Hook.
func (z *ZerologRedisHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		z.logger.Debug().Any("cmds", cmds).Msg("process cmds hook")

		return next(ctx, cmds)
	}
}

var _ redis.Hook = (*ZerologRedisHook)(nil)
