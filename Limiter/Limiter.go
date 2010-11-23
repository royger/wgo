package limiter

import(
	"time"
	"os"
	"sync"
	//"log"
)

const(
	NS_PER_S = 1000000000
)

type limiter struct {
	reset *time.Ticker
	up_mutex *sync.Mutex
	down_mutex *sync.Mutex
	upload, download, up_reset, down_reset, wait_upload, wait_download int64
	up_chan, down_chan chan bool
}

type Limiter interface {
	WaitSend(size int64) int64
	WaitReceive(size int64) int64
}

func NewLimiter(up_limit, down_limit int) (Limiter, os.Error) {
	l := new(limiter)
	l.upload, l.up_reset, l.download, l.down_reset = -1, -1, -1, -1
	if up_limit > 0 || down_limit > 0 {
		l.reset = time.NewTicker(NS_PER_S)
		if up_limit > 0 {
			l.up_mutex = new(sync.Mutex)
			l.up_chan = make(chan bool)
			l.upload, l.up_reset = int64(up_limit)*1000, int64(up_limit)*1000
		}
		if down_limit > 0 {
			l.down_mutex = new(sync.Mutex)
			l.down_chan = make(chan bool)
			l.download, l.down_reset = int64(down_limit)*1000, int64(down_limit)*1000
		}
		go l.run()
	}
	return l, nil
}

func (l *limiter) WaitSend(size int64) int64 {
	if l.upload != -1 {
		l.up_mutex.Lock()
		defer l.up_mutex.Unlock()
		for(l.upload == 0) {
			l.wait_upload++
			l.up_mutex.Unlock()
			<- l.up_chan
			l.up_mutex.Lock()
		}
		left := l.upload - size
		if left < 0 {
			l.upload = 0
			return size + left
		} else {
			l.upload -= size
			return size
		}
	}
	return size
}

func (l *limiter) WaitReceive(size int64) int64 {
	if l.download != -1 {
		l.down_mutex.Lock()
		defer l.down_mutex.Unlock()
		for(l.download == 0) {
			l.wait_download++
			l.down_mutex.Unlock()
			<- l.down_chan
			l.down_mutex.Lock()
		}
		left := l.download - size
		if left < 0 {
			l.download = 0
			return size + left
		} else {
			l.download -= size
			return size
		}
	}
	return size
}

func (l *limiter) run() {
	for {
		select {
		case <- l.reset.C:
			// Reset upload limit
			l.up_mutex.Lock()
			l.upload = l.up_reset
			// Wake up waiting threads
			for ; l.wait_upload > 0; l.wait_upload-- { l.up_chan <- true }
			l.up_mutex.Unlock()
			// Reset download limit
			l.down_mutex.Lock()
			l.download = l.down_reset
			// Wake up waiting threads
			for ; l.wait_download > 0; l.wait_download-- { l.down_chan <- true }
			l.down_mutex.Unlock()
		}
	}
} 