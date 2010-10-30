package limiter

import(
	"time"
	"os"
	//"log"
)

const(
	NS_PER_S = 1000000000
)

type limiter struct {
	upload *time.Ticker
	download *time.Ticker
}

type Limiter interface {
	WaitSend(size int64)
	WaitReceive(size int64)
}

func NewLimiter(up_limit, down_limit int) (Limiter, os.Error) {
	l := new(limiter)
	if up_limit != 0 {
		//log.Println("Limiter -> Upload ticks:", int64(float64(1)/float64(up_limit)*float64(NS_PER_S)))
		l.upload = time.NewTicker(int64(float64(1)/float64(up_limit)*float64(NS_PER_S)))
	}
	if down_limit != 0 {
		//log.Println("Limiter -> Download ticks:", int64(float64(1)/float64(down_limit)*float64(NS_PER_S)))
		l.download = time.NewTicker(int64(float64(1)/float64(down_limit)*float64(NS_PER_S)))
	}
	return l, nil
}

func (l *limiter) WaitSend(size int64) {
	if l.upload != nil {
		limit := int(float64(size)/float64(1000)+0.5)
		for i := 0; i < limit; i++ {
			<- l.upload.C
		}
	}
}

func (l *limiter) WaitReceive(size int64) {
	if l.download != nil {
		limit := int(float64(size)/float64(1000)+0.5)
		for i := 0; i < limit; i++ {
			<- l.download.C
		}
	}
}