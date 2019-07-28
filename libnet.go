package libnet

import (
	"github.com/gobwas/ws"
	"net"
	"net/http"
	"net/url"
	"time"
)

type Event struct {
	OnOpen       func(s *Session, values url.Values) bool
	OnMessage    func(s *Session, in []byte) (out []byte)
	OnError      func(s *Session, err error)
	OnClose      func(s *Session)
	ReadDeadline time.Duration
}

func NewEvent() *Event {
	return &Event{ReadDeadline: time.Minute * 5}
}

func (e *Event) handleOpen(s *Session, values url.Values) bool {
	if e.OnOpen != nil {
		return e.OnOpen(s, values)
	}
	return true
}

func (e *Event) handleMessage(s *Session, in []byte) []byte {
	if e.OnMessage != nil {
		return e.OnMessage(s, in)
	}
	return nil
}

func (e *Event) handleError(s *Session, err error) {
	if e.OnError != nil {
		e.OnError(s, err)
	}
}

func (e *Event) handleClose(s *Session) {
	if e.OnClose != nil {
		e.OnClose(s)
	}
}

func (e *Event) ServeTcp(port string) {
	listener, err := net.Listen("tcp", port)
	if err != nil {
		panic(err.Error())
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		session := newSession(conn, e)
		go session.tcpListener()
		if !e.handleOpen(session, nil) {
			session.Close()
			return
		}
	}
}

func (e *Event) ServeWithHttpRequest(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	conn, _, _, err := ws.UpgradeHTTP(r, w)
	if err != nil {
		return
	}
	session := newSession(conn, e)
	go session.wsListener()
	if !e.handleOpen(session, r.Form) {
		session.Close()
		return
	}
}
