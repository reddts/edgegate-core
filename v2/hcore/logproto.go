package hcore

import (
	"fmt"
	"os"
	"time"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common/observable"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func NewObserver[T any](listenerBufferSize int) *observable.Observer[T] {
	return observable.NewObserver(observable.NewSubscriber[T](listenerBufferSize), listenerBufferSize)
}

func Log(level LogLevel, typ LogType, message ...any) {
	if true || level != LogLevel_DEBUG {
		log.Debug(level, typ, fmt.Sprint(message...))
		fmt.Printf("%v %v %v\n", level, typ, fmt.Sprint(message...))
		os.Stderr.WriteString(fmt.Sprintf("%v %v %v\n", level, typ, fmt.Sprint(message...)))
	}

	static.logObserver.Emit(&LogMessage{
		Level:   level,
		Type:    typ,
		Time:    timestamppb.New(time.Now()),
		Message: fmt.Sprint(message...),
	})
}
