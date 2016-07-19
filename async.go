package routeros

import "github.com/go-routeros/routeros/proto"

type sentenceProcessor interface {
	processSentence(sen *proto.Sentence) (bool, error)
}

type replyCloser interface {
	close(err error)
}

// Async starts asynchronous mode. It returns immediately.
// Run() and Listen() may then be called from multiple goroutines simultaneously.
// Will panic if called multiple times.
func (c *Client) Async() <-chan error {
	c.Lock()
	defer c.Unlock()

	if c.async {
		panic("Async must be called only once")
	}

	c.async = true
	c.tags = make(map[string]sentenceProcessor)
	return c.asyncLoopChan()
}

func (c *Client) asyncLoopChan() <-chan error {
	errC := make(chan error, 1)
	go func() {
		defer close(errC)
		// If c.Close() has been called, c.closing will be true, and
		// err will be “use of closed network connection”. Ignore that error.
		err := c.asyncLoop()
		if err != nil && !c.closing {
			errC <- err
		}
	}()
	return errC
}

func (c *Client) asyncLoop() error {
	for {
		sen, err := c.r.ReadSentence()
		if err != nil {
			c.closeTags(err)
			return err
		}

		c.Lock()
		r, ok := c.tags[sen.Tag]
		c.Unlock()
		if !ok {
			continue
		}

		done, err := r.processSentence(sen)
		if done || err != nil {
			c.Lock()
			delete(c.tags, sen.Tag)
			c.Unlock()
			closeReply(r, err)
		}
	}
}

func (c *Client) closeTags(err error) {
	c.Lock()
	defer c.Unlock()

	for _, r := range c.tags {
		closeReply(r, err)
	}
	c.tags = nil
}

func closeReply(r sentenceProcessor, err error) {
	rr, ok := r.(replyCloser)
	if ok {
		rr.close(err)
	}
}