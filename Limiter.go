package main

import(
	"time"
	"os"
	"log"
)

type Limiter struct {
	upload *time.Ticker
	download *time.Ticker
}

func NewLimiter(up_limit, down_limit int) (l *Limiter, err os.Error) {
	l = new(Limiter)
	if up_limit != 0 {
		log.Stderr("Limiter -> Upload ticks:", int64(float64(1)/float64(up_limit)*float64(NS_PER_S)))
		l.upload = time.NewTicker(int64(float64(1)/float64(up_limit)*float64(NS_PER_S)))
	}
	if down_limit != 0 {
		log.Stderr("Limiter -> Download ticks:", int64(float64(1)/float64(down_limit)*float64(NS_PER_S)))
		l.download = time.NewTicker(int64(float64(1)/float64(down_limit)*float64(NS_PER_S)))
	}
	return
}
