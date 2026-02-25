package scheduler

import "time"

// RetryPolicy controls task retry scheduling.
type RetryPolicy struct {
	BaseDelay time.Duration
	MaxDelay  time.Duration
}

func (p RetryPolicy) nextDelay(attempt int) time.Duration {
	if attempt <= 0 {
		attempt = 1
	}
	if p.BaseDelay <= 0 {
		p.BaseDelay = 200 * time.Millisecond
	}
	if p.MaxDelay <= 0 {
		p.MaxDelay = 5 * time.Second
	}
	delay := p.BaseDelay << (attempt - 1)
	if delay > p.MaxDelay {
		return p.MaxDelay
	}
	return delay
}
