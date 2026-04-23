package hcore

import (
	"context"
	"errors"

	B "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/common/urltest"
)

// BoxService is a lightweight compatibility wrapper used by legacy hcore code.
type BoxService struct {
	ctx            context.Context
	cancel         context.CancelFunc
	instance       *B.Box
	urlTestHistory *urltest.HistoryStorage
}

func NewBoxServiceCompat(
	ctx context.Context,
	cancel context.CancelFunc,
	instance *B.Box,
	urlTestHistory *urltest.HistoryStorage,
) *BoxService {
	return &BoxService{
		ctx:            ctx,
		cancel:         cancel,
		instance:       instance,
		urlTestHistory: urlTestHistory,
	}
}

func (s *BoxService) Start() error {
	return s.instance.Start()
}

func (s *BoxService) Close() error {
	var err error
	if s.urlTestHistory != nil {
		err = s.urlTestHistory.Close()
	}
	if s.instance != nil {
		err = errors.Join(err, s.instance.Close())
	}
	if s.cancel != nil {
		s.cancel()
	}
	return err
}

func (s *BoxService) GetInstance() *B.Box {
	return s.instance
}

func (s *BoxService) UrlTestHistory() *urltest.HistoryStorage {
	return s.urlTestHistory
}

func (s *BoxService) Context() context.Context {
	return s.ctx
}
