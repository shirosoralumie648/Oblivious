package relay

import (
	"github.com/gin-gonic/gin"
	"oblivious/server/internal/relay/handler"
	"oblivious/server/internal/relay/types"
)

type Relay struct {
	engine   *gin.Engine
	pool     *ChannelPool
	handlers map[types.APIType]handler.Handler
}

type Config struct {
	Pool *ChannelPool
}

func NewRelay(cfg *Config) (*Relay, error) {
	r := &Relay{
		handlers: make(map[types.APIType]handler.Handler),
	}
	if cfg != nil && cfg.Pool != nil {
		r.pool = cfg.Pool
	} else {
		r.pool = NewChannelPool()
	}
	r.initRouter()
	return r, nil
}

func (r *Relay) initRouter() {
	r.engine = gin.New()
	handler.RegisterRoutes(r.engine, r.handlers)
}

func (r *Relay) Engine() *gin.Engine {
	return r.engine
}

func (r *Relay) Run(addr string) error {
	return r.engine.Run(addr)
}
