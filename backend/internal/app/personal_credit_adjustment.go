package app

import (
	"context"
	"errors"
	"strings"

	"yingce/backend/internal/model"
	"yingce/backend/internal/repository"
)

type PersonalCreditAdjustmentRequest struct {
	ChannelID          string `json:"channelId"`
	AmountMicrocredits int64  `json:"amountMicrocredits"`
	Note               string `json:"note"`
}

func (s *Service) AdjustPersonalCredits(ctx context.Context, user *model.User, req PersonalCreditAdjustmentRequest) (*model.CreditAccount, error) {
	if user == nil || user.ID == "" {
		return nil, Unauthorized("请先登录")
	}
	if err := s.RequireFeature(FeatureCredits); err != nil {
		return nil, err
	}
	amount := req.AmountMicrocredits
	if amount == 0 || amount < -model.MaxPersonalCreditBalance || amount > model.MaxPersonalCreditBalance {
		return nil, BadAuthRequest("调账积分为 0 或超出允许范围")
	}
	if err := s.checkFixedChannelForAdjustment(ctx, user.ID, strings.TrimSpace(req.ChannelID)); err != nil {
		return nil, err
	}
	account, err := s.repo.AdjustPersonalCredits(user.ID, amount, truncateRunes(strings.TrimSpace(req.Note), 500))
	switch {
	case errors.Is(err, repository.ErrInsufficientCredits):
		return nil, BadAuthRequest("可用积分不足，不能执行本次扣减")
	case errors.Is(err, repository.ErrCreditBalanceLimit):
		return nil, BadAuthRequest("个人积分余额超出上限")
	default:
		return account, err
	}
}
