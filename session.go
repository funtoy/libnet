package libnet

import (
	"encoding/binary"
	"fmt"
	"github.com/funtoy/log"
	"github.com/funtoy/utils"
	"github.com/gobwas/ws/wsutil"
	"io"
	"net"
	"sync/atomic"
	"time"
)

var index int64

const chanSizeOut = 1000

type Session struct {
	sid          int64
	conn         net.Conn
	context      interface{}
	eventHandler *Event
	outChan      chan []byte
	offline      int32
}

func newSession(conn net.Conn, e *Event) *Session {
	return &Session{
		sid:          atomic.AddInt64(&index, 1),
		conn:         conn,
		eventHandler: e,
		outChan:      make(chan []byte, chanSizeOut),
	}
}

func (s *Session) Id() int64 {
	return s.sid
}

func (s *Session) Context() interface{} {
	return s.context
}

func (s *Session) SetContext(context interface{}) {
	s.context = context
}

func (s *Session) Send(data []byte) {
	if s.Online() && len(data) > 0 {
		s.outChan <- data
	}
}

func (s *Session) Online() bool {
	return atomic.LoadInt32(&s.offline) == 0
}

func (s *Session) Close() {
	s.conn.Close()
}

func (s *Session) doClose() {
	atomic.AddInt32(&s.offline, 1)
}

func (s *Session) wsListener() {
	chExit := make(chan struct{})
	defer func() {
		s.doClose()
		chExit <- struct{}{}

		if err := recover(); err != nil {
			log.Errorf("[Recovery] panic ws recovered:%s\n%s", err, utils.Stack(3))
		}
	}()

	go func() {
		for {
			select {
			case msg := <-s.outChan:
				if err := wsutil.WriteServerBinary(s.conn, msg); err != nil {
					s.eventHandler.handleError(s, fmt.Errorf("[writer]ws.WriteServerBinary:%v", err.Error()))
				}

			case <-chExit:
				return

			}
		}
	}()

	if err := s.conn.SetReadDeadline(time.Now().Add(s.eventHandler.ReadDeadline)); err != nil {
		s.eventHandler.handleError(s, fmt.Errorf("[start]ws.SetReadDeadline:%v", err.Error()))
		return
	}

	for {

		msg, opCode, err := wsutil.ReadClientData(s.conn)
		if err != nil {
			s.eventHandler.handleError(s, fmt.Errorf("[looping]ws.ReadClientData:%v", err.Error()))
			return
		}
		if err = s.conn.SetReadDeadline(time.Now().Add(s.eventHandler.ReadDeadline)); err != nil {
			s.eventHandler.handleError(s, fmt.Errorf("[looping]ws.SetReadDeadline:%v", err.Error()))
			return
		}
		out := s.eventHandler.handleMessage(s, msg)
		if len(out) > 0 {
			if err := wsutil.WriteServerMessage(s.conn, opCode, out); err != nil {
				s.eventHandler.handleError(s, fmt.Errorf("[looping]ws.WriteServerMessage:%v", err.Error()))
			}
		}
	}
}

func (s *Session) tcpListener() {
	chExit := make(chan struct{})
	defer func() {
		s.doClose()
		chExit <- struct{}{}

		if err := recover(); err != nil {
			log.Errorf("[Recovery] panic tcp recovered:%s\n%s", err, utils.Stack(3))
		}
	}()

	go func() {
		for {
			select {
			case msg := <-s.outChan:
				if _, err := s.conn.Write(utils.PackOne(msg)); err != nil {
					s.eventHandler.handleError(s, fmt.Errorf("[writer]tcp.Write:%v", err.Error()))
				}

			case <-chExit:
				return

			}
		}
	}()

	if err := s.conn.SetReadDeadline(time.Now().Add(s.eventHandler.ReadDeadline)); err != nil {
		s.eventHandler.handleError(s, fmt.Errorf("[start]tcp.SetReadDeadline:%v", err.Error()))
		return
	}

	var head = make([]byte, 4)
	for {

		if _, err := io.ReadFull(s.conn, head); err != nil {
			s.eventHandler.handleError(s, fmt.Errorf("[looping]tcp.read.header:%v", err.Error()))
			return
		}
		if err := s.conn.SetReadDeadline(time.Now().Add(s.eventHandler.ReadDeadline)); err != nil {
			s.eventHandler.handleError(s, fmt.Errorf("[looping]tcp.SetReadDeadline:%v", err.Error()))
			return
		}

		size := binary.BigEndian.Uint32(head)
		data := make([]byte, size)
		if _, err := io.ReadFull(s.conn, data); err != nil {
			s.eventHandler.handleError(s, fmt.Errorf("[looping]tcp.read.data:%v", err.Error()))
			return
		}
		out := s.eventHandler.handleMessage(s, data)
		if len(out) > 0 {
			_, err := s.conn.Write(out)
			if err != nil {
				s.eventHandler.handleError(s, fmt.Errorf("[looping]tcp.Write:%v", err.Error()))
			}
		}
	}
}
