package topupstore

import (
	"errors"
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	infradb "github.com/NookMux/NookMux/internal/infra/db"
	"github.com/NookMux/NookMux/internal/infra/log"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/token"
	"github.com/NookMux/NookMux/internal/store/user"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TopUp struct {
	Id              int     `json:"id"`
	UserId          int     `json:"user_id" gorm:"index"`
	Amount          int64   `json:"amount"`
	Money           float64 `json:"money"`
	TradeNo         string  `json:"trade_no" gorm:"unique;type:varchar(255);index"`
	PaymentMethod   string  `json:"payment_method" gorm:"type:varchar(50)"`
	PaymentProvider string  `json:"payment_provider" gorm:"type:varchar(50);default:''"`
	CreateTime      int64   `json:"create_time"`
	CompleteTime    int64   `json:"complete_time"`
	Status          string  `json:"status"`
}

const (
	PaymentMethodStripe = "stripe"
)

const (
	PaymentProviderEpay   = "epay"
	PaymentProviderStripe = "stripe"
)

// CallbackPaymentMethodAdmin 标记该充值由管理员手动补单完成，而非支付网关回调。
const CallbackPaymentMethodAdmin = "admin"

var (
	ErrPaymentProviderMismatch = errors.New("payment provider mismatch")
	ErrPaymentMethodMismatch   = errors.New("payment method mismatch")
	ErrPaymentAmountMismatch   = errors.New("payment amount mismatch")
	ErrTopUpNotFound           = errors.New("topup not found")
	ErrTopUpStatusInvalid      = errors.New("topup status invalid")
)

func (topUp *TopUp) Insert() error {
	err := dbstore.DB.Create(topUp).Error
	return err
}

func (topUp *TopUp) Update() error {
	err := dbstore.DB.Save(topUp).Error
	return err
}

func GetTopUpById(id int) *TopUp {
	var topUp *TopUp
	err := dbstore.DB.Where("id = ?", id).First(&topUp).Error
	if err != nil {
		return nil
	}
	return topUp
}

// GetTopUpByTradeNo 按订单号查询充值订单；订单不存在时返回 (nil, nil)，
// 其余数据库错误原样上抛，由调用方区分处理。
func GetTopUpByTradeNo(tradeNo string) (*TopUp, error) {
	var topUp *TopUp
	err := dbstore.DB.Where("trade_no = ?", tradeNo).First(&topUp).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return topUp, nil
}

func validateTopUpCallback(topUp *TopUp, expectedProvider string, expectedMethod string, expectedMoney string) error {
	if expectedProvider != "" && topUp.PaymentProvider != expectedProvider {
		return ErrPaymentProviderMismatch
	}
	if expectedMethod != "" && topUp.PaymentMethod != expectedMethod {
		return ErrPaymentMethodMismatch
	}
	if expectedMoney != "" {
		paidMoney, err := decimal.NewFromString(expectedMoney)
		if err != nil {
			return ErrPaymentAmountMismatch
		}
		orderMoney := decimal.NewFromFloat(topUp.Money).Round(2)
		if !paidMoney.Round(2).Equal(orderMoney) {
			return ErrPaymentAmountMismatch
		}
	}
	if topUp.Status != common.TopUpStatusPending {
		return ErrTopUpStatusInvalid
	}
	return nil
}

func UpdatePendingTopUpStatus(tradeNo string, expectedPaymentProvider string, targetStatus string) error {
	if tradeNo == "" {
		return errors.New("未提供支付单号")
	}

	refCol := "`trade_no`"
	if infradb.UsingPostgreSQL {
		refCol = `"trade_no"`
	}

	return dbstore.DB.Transaction(func(tx *gorm.DB) error {
		topUp := &TopUp{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(refCol+" = ?", tradeNo).First(topUp).Error; err != nil {
			return ErrTopUpNotFound
		}
		if err := validateTopUpCallback(topUp, expectedPaymentProvider, "", ""); err != nil {
			return err
		}

		// Atomic status flip guarded by the pending status. Prevents concurrent
		// callbacks from both transitioning the same order.
		result := tx.Model(&TopUp{}).
			Where("id = ? AND status = ?", topUp.Id, common.TopUpStatusPending).
			Update("status", targetStatus)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrTopUpStatusInvalid
		}
		return nil
	})
}

func Recharge(referenceId string, customerId string, callerIp string) (err error) {
	if referenceId == "" {
		return errors.New("未提供支付单号")
	}

	var quotaToAdd int
	topUp := &TopUp{}

	refCol := "`trade_no`"
	if infradb.UsingPostgreSQL {
		refCol = `"trade_no"`
	}

	err = dbstore.DB.Transaction(func(tx *gorm.DB) error {
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(refCol+" = ?", referenceId).First(topUp).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrTopUpNotFound
			}
			return err
		}

		if err := validateTopUpCallback(topUp, PaymentProviderStripe, PaymentMethodStripe, ""); err != nil {
			return err
		}

		// 额度换算走带范围检查的 decimal 路径：超界（如 32 位构建下大额充值）
		// 显式报错回滚，不截断入账，也避免 float 参数在 PostgreSQL 22P02 /
		// SQLite REAL 化的兼容问题。
		quotaDecimal := decimal.NewFromFloat(topUp.Money).Mul(decimal.NewFromFloat(common.QuotaPerUnit))
		var cerr error
		quotaToAdd, cerr = common.DecimalToInt(quotaDecimal)
		if cerr != nil {
			return fmt.Errorf("充值额度超出平台 int 可表示范围（money=%.2f）: %w", topUp.Money, cerr)
		}
		if quotaToAdd <= 0 {
			return errors.New("无效的充值额度")
		}

		// Atomic status flip: only succeed if the order is still pending.
		// This is the authoritative guard against concurrent callbacks
		// double-crediting the same order.
		result := tx.Model(&TopUp{}).
			Where("id = ? AND status = ?", topUp.Id, common.TopUpStatusPending).
			Updates(map[string]interface{}{
				"status":        common.TopUpStatusSuccess,
				"complete_time": common.GetTimestamp(),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrTopUpStatusInvalid
		}

		err = tx.Model(&userstore.User{}).Where("id = ?", topUp.UserId).Updates(map[string]interface{}{"stripe_customer": customerId, "quota": gorm.Expr("quota + ?", quotaToAdd)}).Error
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		common.SysError("topup failed: " + err.Error())
		// 保留 sentinel 类型，供 Stripe webhook 侧 errors.Is 区分：
		// ErrTopUpStatusInvalid = 订单已处理（重复投递）；
		// ErrTopUpNotFound = 订单不存在（重投也无法出现，属终态）。
		if errors.Is(err, ErrTopUpStatusInvalid) || errors.Is(err, ErrTopUpNotFound) {
			return err
		}
		return errors.New("充值失败，请稍后重试")
	}

	userstore.RecordTopupLog(topUp.UserId, fmt.Sprintf("使用在线充值成功，充值金额: %v，支付金额：%d", log.FormatQuota(quotaToAdd), topUp.Amount), callerIp, topUp.PaymentMethod, PaymentProviderStripe)

	return nil
}

func CompleteEpayTopUp(tradeNo string, paymentMethod string, paidMoney string, callerIp string) error {
	if tradeNo == "" {
		return errors.New("未提供订单号")
	}

	refCol := "`trade_no`"
	if infradb.UsingPostgreSQL {
		refCol = `"trade_no"`
	}

	var userId int
	var quotaToAdd int
	var payMoney float64
	var orderPaymentMethod string

	err := dbstore.DB.Transaction(func(tx *gorm.DB) error {
		topUp := &TopUp{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(refCol+" = ?", tradeNo).First(topUp).Error; err != nil {
			return ErrTopUpNotFound
		}
		if err := validateTopUpCallback(topUp, PaymentProviderEpay, paymentMethod, paidMoney); err != nil {
			return err
		}

		dAmount := decimal.NewFromInt(topUp.Amount)
		dQuotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)
		var cerr error
		quotaToAdd, cerr = common.DecimalToInt(dAmount.Mul(dQuotaPerUnit))
		if cerr != nil {
			// 32 位构建下大额充值会超出 int 表示范围（回绕后静默少入账），显式报错回滚。
			return fmt.Errorf("充值额度超出平台 int 可表示范围（amount=%d）: %w", topUp.Amount, cerr)
		}
		if quotaToAdd <= 0 {
			return errors.New("无效的充值额度")
		}

		// Atomic status flip: only succeed if the order is still pending,
		// preventing concurrent callbacks from double-crediting.
		result := tx.Model(&TopUp{}).
			Where("id = ? AND status = ?", topUp.Id, common.TopUpStatusPending).
			Updates(map[string]interface{}{
				"status":        common.TopUpStatusSuccess,
				"complete_time": common.GetTimestamp(),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrTopUpStatusInvalid
		}

		if err := tx.Model(&userstore.User{}).Where("id = ?", topUp.UserId).Update("quota", gorm.Expr("quota + ?", quotaToAdd)).Error; err != nil {
			return err
		}

		userId = topUp.UserId
		payMoney = topUp.Money
		orderPaymentMethod = topUp.PaymentMethod
		return nil
	})
	if err != nil {
		return err
	}

	userstore.RecordTopupLog(userId, fmt.Sprintf("使用在线充值成功，充值金额: %v，支付金额：%f", log.LogQuota(quotaToAdd), payMoney), callerIp, orderPaymentMethod, PaymentProviderEpay)
	return nil
}

const topUpQueryWindowSeconds int64 = 30 * 24 * 60 * 60

func topUpQueryCutoff() int64 {
	return common.GetTimestamp() - topUpQueryWindowSeconds
}

// userTopUpListWhere 普通用户订单列表的过滤条件：历史记录限制在时间窗口内，
// 但待支付订单不受窗口限制——回调丢失的滞留订单需保持可见，供用户自查支付状态。
const userTopUpListWhere = "user_id = ? AND (create_time >= ? OR status = ?)"

func GetUserTopUps(userId int, pageInfo *common.PageInfo) (topups []*TopUp, total int64, err error) {
	// Start transaction
	tx := dbstore.DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	cutoff := topUpQueryCutoff()

	// Get total count within transaction
	err = tx.Model(&TopUp{}).Where(userTopUpListWhere, userId, cutoff, common.TopUpStatusPending).Count(&total).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// Get paginated topups within same transaction
	err = tx.Where(userTopUpListWhere, userId, cutoff, common.TopUpStatusPending).Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&topups).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// Commit transaction
	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}

	return topups, total, nil
}

// GetAllTopUps 获取全平台的充值记录（管理员使用，不限制时间窗口）
func GetAllTopUps(pageInfo *common.PageInfo) (topups []*TopUp, total int64, err error) {
	tx := dbstore.DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err = tx.Model(&TopUp{}).Count(&total).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	if err = tx.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&topups).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}

	return topups, total, nil
}

const searchTopUpCountHardLimit = 10000

// SearchUserTopUps 按订单号搜索某用户的充值记录
func SearchUserTopUps(userId int, keyword string, pageInfo *common.PageInfo) (topups []*TopUp, total int64, err error) {
	tx := dbstore.DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	query := tx.Model(&TopUp{}).Where(userTopUpListWhere, userId, topUpQueryCutoff(), common.TopUpStatusPending)
	if keyword != "" {
		pattern, perr := tokenstore.SanitizeLikePattern(keyword)
		if perr != nil {
			tx.Rollback()
			return nil, 0, perr
		}
		query = query.Where("trade_no LIKE ? ESCAPE '!'", pattern)
	}

	if err = query.Limit(searchTopUpCountHardLimit).Count(&total).Error; err != nil {
		tx.Rollback()
		common.SysError("failed to count search topups: " + err.Error())
		return nil, 0, errors.New("搜索充值记录失败")
	}

	if err = query.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&topups).Error; err != nil {
		tx.Rollback()
		common.SysError("failed to search topups: " + err.Error())
		return nil, 0, errors.New("搜索充值记录失败")
	}

	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}
	return topups, total, nil
}

// SearchAllTopUps 按订单号搜索全平台充值记录（管理员使用，不限制时间窗口）
func SearchAllTopUps(keyword string, pageInfo *common.PageInfo) (topups []*TopUp, total int64, err error) {
	tx := dbstore.DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	query := tx.Model(&TopUp{})
	if keyword != "" {
		pattern, perr := tokenstore.SanitizeLikePattern(keyword)
		if perr != nil {
			tx.Rollback()
			return nil, 0, perr
		}
		query = query.Where("trade_no LIKE ? ESCAPE '!'", pattern)
	}

	if err = query.Limit(searchTopUpCountHardLimit).Count(&total).Error; err != nil {
		tx.Rollback()
		common.SysError("failed to count search topups: " + err.Error())
		return nil, 0, errors.New("搜索充值记录失败")
	}

	if err = query.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&topups).Error; err != nil {
		tx.Rollback()
		common.SysError("failed to search topups: " + err.Error())
		return nil, 0, errors.New("搜索充值记录失败")
	}

	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}
	return topups, total, nil
}

// ManualCompleteTopUp 管理员手动完成订单并给用户充值
func ManualCompleteTopUp(tradeNo string, callerIp string) error {
	if tradeNo == "" {
		return errors.New("未提供订单号")
	}

	refCol := "`trade_no`"
	if infradb.UsingPostgreSQL {
		refCol = `"trade_no"`
	}

	var userId int
	var quotaToAdd int
	var payMoney float64
	var paymentMethod string
	var credited bool

	err := dbstore.DB.Transaction(func(tx *gorm.DB) error {
		topUp := &TopUp{}
		// 行级锁，避免并发补单
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(refCol+" = ?", tradeNo).First(topUp).Error; err != nil {
			return errors.New("充值订单不存在")
		}

		// 幂等处理：已成功直接返回
		if topUp.Status == common.TopUpStatusSuccess {
			return nil
		}

		if topUp.Status != common.TopUpStatusPending {
			return errors.New("订单状态不是待支付，无法补单")
		}

		// 计算应充值额度：
		// - Stripe 订单：Money 代表经分组倍率换算后的美元数量，直接 * QuotaPerUnit
		// - 其他订单（如易支付）：Amount 为美元数量，* QuotaPerUnit
		dQuotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)
		var quotaDecimal decimal.Decimal
		if topUp.PaymentMethod == PaymentMethodStripe {
			quotaDecimal = decimal.NewFromFloat(topUp.Money).Mul(dQuotaPerUnit)
		} else {
			quotaDecimal = decimal.NewFromInt(topUp.Amount).Mul(dQuotaPerUnit)
		}
		var cerr error
		quotaToAdd, cerr = common.DecimalToInt(quotaDecimal)
		if cerr != nil {
			// 32 位构建下大额充值会超出 int 表示范围（回绕后静默少入账），显式报错回滚。
			return fmt.Errorf("充值额度超出平台 int 可表示范围（money=%.2f, amount=%d）: %w", topUp.Money, topUp.Amount, cerr)
		}
		if quotaToAdd <= 0 {
			return errors.New("无效的充值额度")
		}

		// 原子状态翻转：仅当订单仍为待支付时才标记完成，避免并发补单重复入账
		result := tx.Model(&TopUp{}).
			Where("id = ? AND status = ?", topUp.Id, common.TopUpStatusPending).
			Updates(map[string]interface{}{
				"status":        common.TopUpStatusSuccess,
				"complete_time": common.GetTimestamp(),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			// 已被其他并发请求处理，视为成功（幂等）
			return nil
		}

		// 增加用户额度（立即写库，保持一致性）
		if err := tx.Model(&userstore.User{}).Where("id = ?", topUp.UserId).Update("quota", gorm.Expr("quota + ?", quotaToAdd)).Error; err != nil {
			return err
		}

		userId = topUp.UserId
		payMoney = topUp.Money
		paymentMethod = topUp.PaymentMethod
		credited = true
		return nil
	})

	if err != nil {
		return err
	}

	if !credited {
		// 订单此前已完成或已被并发请求处理，本次没有产生新的入账，不记录补单日志。
		return nil
	}

	// 事务外记录日志，避免阻塞
	userstore.RecordTopupLog(userId, fmt.Sprintf("管理员补单成功，充值金额: %v，支付金额：%f", log.FormatQuota(quotaToAdd), payMoney), callerIp, paymentMethod, CallbackPaymentMethodAdmin)
	return nil
}
