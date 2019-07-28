package libnet_test

import (
	"fmt"
	"github.com/funtoy/libnet"
	"github.com/gin-gonic/gin"
	"net/url"
	"runtime"
	"sync/atomic"
	"testing"
)

var id int32

func TestEvent_Serve(t *testing.T) {
	e := libnet.NewEvent()
	e.OnOpen = func(s *libnet.Session, v url.Values) bool {
		index := atomic.AddInt32(&id, 1)
		s.SetContext(index)
		fmt.Printf("%v open with args:%v\n", s.Context(), v)

		if index%100 == 0 {
			fmt.Printf("Total number of connections: %v ", index)

			mem := new(runtime.MemStats)
			runtime.ReadMemStats(mem)
			fmt.Printf("Mem: %v\n", mem)
		}

		s.Send([]byte("ok"))

		return true
	}
	e.OnMessage = func(s *libnet.Session, in []byte) []byte {
		fmt.Printf("%v receive:%v\n", s.Context(), string(in))
		return in
	}
	e.OnClose = func(s *libnet.Session) {
		fmt.Printf("%v closed\n", s.Context())
	}
	e.OnError = func(s *libnet.Session, err error) {
		fmt.Printf("%v OnError:%v\n", s.Context(), err.Error())
	}

	gin.SetMode(gin.ReleaseMode)
	g := gin.New()

	g.Use(gin.Recovery())
	g.GET("/", func(c *gin.Context) {
		e.ServeWithHttpRequest(c.Writer, c.Request)
	})

	panic(g.Run(":8000"))
}
