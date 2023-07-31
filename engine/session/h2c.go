package session

import (
	context "context"
	"errors"
	"io"
	"net"
	"sync"

	"github.com/moby/buildkit/util/grpcerrors"
	"github.com/vito/progrock"
	"google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
)

type ProxyListenerAttachable struct {
	rec *progrock.Recorder
	UnimplementedProxyListenerServer
}

func NewProxyListenerAttachable(rec *progrock.Recorder) ProxyListenerAttachable {
	return ProxyListenerAttachable{
		rec: rec,
	}
}

func (s ProxyListenerAttachable) Register(srv *grpc.Server) {
	RegisterProxyListenerServer(srv, &s)
}

func (s ProxyListenerAttachable) Listen(srv ProxyListener_ListenServer) error {
	req, err := srv.Recv()
	if err != nil {
		return err
	}

	l, err := net.Listen(req.GetProtocol(), req.GetAddr())
	if err != nil {
		return err
	}
	defer l.Close()

	err = srv.Send(&ListenResponse{
		Addr: l.Addr().String(),
	})
	if err != nil {
		return err
	}

	conns := map[string]net.Conn{}
	connsL := &sync.Mutex{}
	sendL := &sync.Mutex{}

	defer func() {
		connsL.Lock()
		for _, conn := range conns {
			conn.Close()
		}
		connsL.Unlock()
	}()

	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				if !errors.Is(err, net.ErrClosed) {
					s.rec.Warn("accept error", progrock.ErrorLabel(err))
				}
				return
			}

			connId := conn.RemoteAddr().String()

			connsL.Lock()
			conns[connId] = conn
			connsL.Unlock()

			sendL.Lock()
			err = srv.Send(&ListenResponse{
				ConnId: connId,
			})
			sendL.Unlock()
			if err != nil {
				s.rec.Warn("send connId error", progrock.ErrorLabel(err))
				return
			}

			go func() {
				for {
					// Read data from the connection
					data := make([]byte, 1024)
					n, err := conn.Read(data)
					if err != nil {
						if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
							// conn closed
							return
						}

						s.rec.Warn("conn read error", progrock.ErrorLabel(err))
						return
					}

					sendL.Lock()
					err = srv.Send(&ListenResponse{
						ConnId: connId,
						Data:   data[:n],
					})
					sendL.Unlock()
					if err != nil {
						s.rec.Warn("listener send response error", progrock.ErrorLabel(err))
						return
					}
				}
			}()
		}
	}()

	for {
		req, err := srv.Recv()
		if err != nil {
			if errors.Is(err, context.Canceled) || grpcerrors.Code(err) == codes.Canceled {
				// canceled
				return nil
			}

			if errors.Is(err, io.EOF) {
				// stopped
				return nil
			}

			if grpcerrors.Code(err) == codes.Unavailable {
				// client disconnected (i.e. quitting Dagger out)
				return nil
			}

			s.rec.Error("listener receive request error", progrock.ErrorLabel(err))
			return err
		}

		connId := req.GetConnId()
		if req.GetConnId() == "" {
			s.rec.Warn("listener request with no connId")
			continue
		}

		connsL.Lock()
		conn, ok := conns[connId]
		connsL.Unlock()
		if !ok {
			s.rec.Warn("listener request for unknown connId", progrock.Labelf("connId", connId))
			continue
		}

		switch {
		case req.GetClose():
			if err := conn.Close(); err != nil {
				s.rec.Warn("conn close error", progrock.ErrorLabel(err))
				continue
			}
			connsL.Lock()
			delete(conns, connId)
			connsL.Unlock()
		case req.Data != nil:
			_, err = conn.Write(req.GetData())
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					// conn closed
					return nil
				}

				s.rec.Warn("conn write error", progrock.ErrorLabel(err))
				continue
			}
		}
	}
}
