package tunnel

import (
	"sort"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"lion/pkg/guacd"
	"lion/pkg/logger"
	"lion/pkg/session"
)

const (
	INTERNALDATAOPCODE = ""
	PINGOPCODE         = "ping"
)

var _ sort.Interface = Connections{}

type Connections []*Connection

func (c Connections) Len() int {
	return len(c)
}
func (c Connections) Less(i, j int) bool {
	iCreated := c[i].Sess.Created.UTC()
	jCreated := c[j].Sess.Created.UTC()
	return iCreated.Before(jCreated)
}

func (c Connections) Swap(i, j int) {
	c[j], c[i] = c[i], c[j]
}

type Connection struct {
	Sess        *session.TunnelSession
	guacdTunnel *guacd.Tunnel

	ws *websocket.Conn

	wsLock    sync.Mutex
	guacdLock sync.Mutex

	outputFilter *OutputStreamInterceptingFilter

	inputFilter *InputStreamInterceptingFilter
}

func (t *Connection) SendWsMessage(msg guacd.Instruction) error {
	return t.writeWsMessage([]byte(msg.String()))
}

func (t *Connection) writeWsMessage(p []byte) error {
	t.wsLock.Lock()
	defer t.wsLock.Unlock()
	return t.ws.WriteMessage(websocket.TextMessage, p)
}

func (t *Connection) WriteTunnelMessage(msg guacd.Instruction) (err error) {
	_, err = t.writeTunnelMessage([]byte(msg.String()))
	return err
}

func (t *Connection) writeTunnelMessage(p []byte) (int, error) {
	t.guacdLock.Lock()
	defer t.guacdLock.Unlock()
	return t.guacdTunnel.WriteAndFlush(p)
}

func (t *Connection) readTunnelInstruction() (*guacd.Instruction, error) {
	for {
		instruction, err := t.guacdTunnel.ReadInstruction()
		if err != nil {
			return nil, err
		}
		newInstruction := t.inputFilter.Filter(&instruction)
		if newInstruction == nil {
			continue
		}
		newInstruction = t.outputFilter.Filter(newInstruction)
		if newInstruction == nil {
			continue
		}
		return newInstruction, nil
	}

}

func (t *Connection) Run(ctx *gin.Context) (err error) {
	// 需要发送 uuid 返回给 guacamole tunnel
	err = t.SendWsMessage(guacd.NewInstruction(
		INTERNALDATAOPCODE, t.guacdTunnel.UUID))
	if err != nil {
		logger.Error("Run err: ", err)
		return err
	}
	exit := make(chan error, 2)
	activeChan := make(chan struct{})
	go func(t *Connection) {
		for {
			instruction, err := t.readTunnelInstruction()
			if err != nil {
				logger.Errorf("Session[%s] guacamole server read err: %+v", t, err)
				exit <- err
				break
			}

			switch instruction.Opcode {
			case guacd.InstructionServerDisconnect,
				guacd.InstructionServerError:
				logger.Infof("Session[%s] receive guacamole server disconnect opcode", t)
			}

			if err = t.writeWsMessage([]byte(instruction.String())); err != nil {
				logger.Errorf("Session[%s] send web client err: %+v", t, err)
				exit <- err
				break
			}
		}
		_ = t.ws.Close()
	}(t)

	go func(t *Connection) {
		for {
			_, message, err := t.ws.ReadMessage()
			if err != nil {
				logger.Errorf("Session[%s] web client read err: %+v", t, err)
				exit <- err
				break
			}

			if ret, err := guacd.ParseInstructionString(string(message)); err == nil {
				if ret.Opcode == INTERNALDATAOPCODE && len(ret.Args) >= 2 && ret.Args[0] == PINGOPCODE {
					if err := t.SendWsMessage(guacd.NewInstruction(INTERNALDATAOPCODE, PINGOPCODE)); err != nil {
						logger.Errorf("Session[%s] unable to send 'ping' response for WebSocket tunnel: %+v",
							t, err)
					}
					continue
				}

				switch ret.Opcode {
				case guacd.InstructionClientSync,
					 guacd.InstructionClientNop,
					 guacd.InstructionStreamingAck:

				case guacd.InstructionClientDisconnect:
					logger.Errorf("Session[%s] receive web client disconnect opcode", t)
				default:
					select {
					case activeChan <- struct{}{}:
					default:
					}
				}
			} else {
				logger.Errorf("Session[%s] parse instruction err %s", t, err)
			}
			_, err = t.writeTunnelMessage(message)
			if err != nil {
				logger.Errorf("Session[%s] guacamole server write err: %+v", t, err)
				exit <- err
				break
			}
		}
	}(t)
	maxIndexTime := t.Sess.TerminalConfig.MaxIdleTime
	maxIdleMinutes := time.Duration(maxIndexTime) * time.Minute
	activeDetectTicker := time.NewTicker(time.Minute)
	defer activeDetectTicker.Stop()
	latestActive := time.Now()
	for {
		select {
		case err = <-exit:
			logger.Infof("Session[%s] Connection exit %+v", t, err)
			return err
		case <-ctx.Request.Context().Done():
			_ = t.ws.Close()
			_ = t.guacdTunnel.Close()
			logger.Errorf("Session[%s] request ctx done", t)
			return nil
		case <-activeChan:
			latestActive = time.Now()
		case detectTime := <-activeDetectTicker.C:
			if detectTime.After(latestActive.Add(maxIdleMinutes)) {
				errInstruction := guacd.NewInstruction(
					guacd.InstructionServerError, "Terminated by timeout ", "1011")
				_ = t.SendWsMessage(errInstruction)
				logger.Errorf("Session[%s] terminated by %d min timeout",
					t, maxIndexTime)
				return nil
			}
		}
	}

}

func (t *Connection) Terminate() {
	errInstruction := guacd.NewInstruction(
		// Todo 定义 code 表明 JMS 终断
		guacd.InstructionServerError, "admin Terminate", "1011")
	_ = t.SendWsMessage(errInstruction)
	logger.Errorf("Session[%s] terminated by Admin", t)
}

func (t *Connection) String() string {
	return t.Sess.ID
}
