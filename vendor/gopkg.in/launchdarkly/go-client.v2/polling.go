package ldclient

import (
	"sync"
	"time"
)

type pollingProcessor struct {
	store              FeatureStore
	requestor          *requestor
	config             Config
	setInitializedOnce sync.Once
	isInitialized      bool
	quit               chan struct{}
	closeOnce          sync.Once
}

func newPollingProcessor(config Config, requestor *requestor) updateProcessor {
	pp := &pollingProcessor{
		store:     config.FeatureStore,
		requestor: requestor,
		config:    config,
		quit:      make(chan struct{}),
	}

	return pp
}

func (pp *pollingProcessor) start(closeWhenReady chan<- struct{}) {
	pp.config.Logger.Printf("Starting LaunchDarkly polling processor with interval: %+v", pp.config.PollInterval)
	go func() {
		for {
			select {
			case <-pp.quit:
				pp.config.Logger.Printf("Polling Processor closed.")
				return
			default:
				then := time.Now()
				err := pp.poll()
				if err == nil {
					pp.setInitializedOnce.Do(func() {
						pp.isInitialized = true
						close(closeWhenReady)
					})
				} else {
					pp.config.Logger.Printf("Error when requesting feature updates: %+v", err)
					if hse, ok := err.(HttpStatusError); ok {
						if hse.Code == 401 {
							pp.config.Logger.Printf("Received 401 error, no further polling requests will be made since SDK key is invalid")
							return
						}
					}
				}
				delta := pp.config.PollInterval - time.Since(then)

				if delta > 0 {
					time.Sleep(delta)
				}
			}
		}
	}()
}

func (pp *pollingProcessor) poll() error {
	features, cached, err := pp.requestor.requestAllFlags()

	if err != nil {
		return err
	}

	// We initialize the store only if the request wasn't cached
	if !cached {
		return pp.store.Init(features)
	}
	return nil
}

func (pp *pollingProcessor) close() {
	pp.closeOnce.Do(func() {
		pp.config.Logger.Printf("Closing Polling Processor")
		close(pp.quit)
	})
}

func (pp *pollingProcessor) initialized() bool {
	return pp.isInitialized
}
