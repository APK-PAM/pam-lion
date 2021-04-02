package tunnel

import (
	"encoding/base64"
	"fmt"
	"io"
	"sync"

	"guacamole-client-go/pkg/guacd"
)

type InputStreamInterceptingFilter struct {
	tunnel  *TunnelConn
	streams map[string]*InputStreamResource
	sync.Mutex
	acknowledgeBlobs bool
}

func (filter *InputStreamInterceptingFilter) Filter(unfilteredInstruction *guacd.Instruction) *guacd.Instruction {

	if unfilteredInstruction.Opcode == guacd.InstructionStreamingAck {
		filter.handleAck(unfilteredInstruction)
	}
	return unfilteredInstruction
}

func (filter *InputStreamInterceptingFilter) handleAck(unfilteredInstruction *guacd.Instruction) {
	filter.Lock()
	defer filter.Unlock()
	// Verify all required arguments are present
	args := unfilteredInstruction.Args
	if len(args) < 3 {
		return
	}
	index := args[0]
	if stream, ok := filter.streams[index]; ok {
		status := args[2]
		if status != "0" {
			return
		}

		// Send next blob
		filter.readNextBlob(stream)

		//stream.reader.Read()
	}
	return
}

func (filter *InputStreamInterceptingFilter) readNextBlob(stream *InputStreamResource) {
	buf := make([]byte, 6048)
	nr, err := stream.reader.Read(buf)
	if nr > 0 {
		filter.sendBlob(stream.streamIndex, buf[:nr])
	}
	if err != nil {
		if err != io.EOF {
			stream.err = err
		}
		filter.closeInterceptedStream(stream.streamIndex)
		return
	}

}

func (filter *InputStreamInterceptingFilter) sendBlob(index string, p []byte) {
	err := filter.tunnel.WriteTunnelMessage(guacd.NewInstruction(
		guacd.InstructionStreamingBlob, index, base64.StdEncoding.EncodeToString(p)))
	if err != nil {
		fmt.Println(err)
	}
}

func (filter *InputStreamInterceptingFilter) closeInterceptedStream(index string) {
	if outStream, ok := filter.streams[index]; ok {
		fmt.Println("closeInterceptedStream index ", index)
		close(outStream.done)
	}
	delete(filter.streams, index)
}

func (filter *InputStreamInterceptingFilter) sendEnd(index string) {
	err := filter.tunnel.WriteTunnelMessage(guacd.NewInstruction(
		guacd.InstructionStreamingEnd, index))
	if err != nil {
		fmt.Println(err)
	}
}

func (filter *InputStreamInterceptingFilter) addInputStream(stream *InputStreamResource) {
	filter.Lock()
	defer filter.Unlock()
	filter.streams[stream.streamIndex] = stream
	filter.readNextBlob(stream)
}

// 上传文件的对象
type InputStreamResource struct {
	streamIndex string
	mediaType   string // application/octet-stream
	reader      io.ReadCloser
	done        chan struct{}

	err error
}

func (r *InputStreamResource) Wait() {
	<-r.done
}

func (r *InputStreamResource) WaitErr() error {
	return r.err
}
