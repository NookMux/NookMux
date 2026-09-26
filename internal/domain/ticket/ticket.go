package ticket

import (
	"errors"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/internal/store/ticket"
	"gorm.io/gorm"
	"strings"
	"unicode/utf8"
)

const (
	maxTicketTitleRunes   = 255
	maxTicketContentRunes = 10000
)

type TicketSummary struct {
	Id        int    `json:"id"`
	Title     string `json:"title"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

type TicketMessage struct {
	Id       int    `json:"id"`
	Type     string `json:"type"`
	Role     string `json:"role"`
	Username string `json:"username"`
	Content  string `json:"content,omitempty"`
	Value    string `json:"value,omitempty"`
	Time     int64  `json:"time"`
}

type TicketDetail struct {
	Id        int             `json:"id"`
	Title     string          `json:"title"`
	Type      string          `json:"type"`
	Status    string          `json:"status"`
	CreatedAt int64           `json:"created_at"`
	UpdatedAt int64           `json:"updated_at"`
	ClosedAt  int64           `json:"closed_at"`
	Messages  []TicketMessage `json:"messages"`
}

type CreateTicketInput struct {
	UserId   int
	Username string
	Role     int
	Title    string
	Type     string
	Content  string
}

type ReplyTicketInput struct {
	TicketId  int
	UserId    int
	Username  string
	Role      int
	Content   string
	NewStatus string
}

func ListUserTickets(userId int, page, pageSize int, status string, keyword string) ([]TicketSummary, int64, error) {
	filter, err := buildTicketListFilter(userId, page, pageSize, status, keyword)
	if err != nil {
		return nil, 0, err
	}
	tickets, total, err := ticketstore.ListUserTickets(filter)
	if err != nil {
		return nil, 0, err
	}
	return buildTicketSummaries(tickets), total, nil
}

func ListAdminTickets(lang string, role int, page, pageSize int, status string, keyword string) ([]TicketSummary, int64, error) {
	if !canManageAllTickets(role) {
		return nil, 0, errors.New(i18n.Translate(lang, i18n.MsgTicketNoPermission))
	}
	filter, err := buildTicketListFilter(0, page, pageSize, status, keyword)
	if err != nil {
		return nil, 0, err
	}
	tickets, total, err := ticketstore.ListAdminTickets(filter)
	if err != nil {
		return nil, 0, err
	}
	return buildTicketSummaries(tickets), total, nil
}

func CreateTicket(lang string, input CreateTicketInput) (*TicketDetail, error) {
	title, err := validateTicketText(lang, input.Title, maxTicketTitleRunes, i18n.MsgTicketTitleEmpty, i18n.MsgTicketTitleTooLong)
	if err != nil {
		return nil, err
	}
	content, err := validateTicketText(lang, input.Content, maxTicketContentRunes, i18n.MsgTicketContentEmpty, i18n.MsgTicketContentTooLong)
	if err != nil {
		return nil, err
	}
	ticketType, err := ticketstore.ParseTicketType(input.Type)
	if err != nil {
		return nil, err
	}

	ticket := ticketstore.NewTicket(title, input.UserId, ticketType)
	entry := &ticketstore.TicketEntry{
		EntryType:    ticketstore.TicketEntryTypeMessage,
		SenderUserId: input.UserId,
		SenderName:   input.Username,
		SenderRole:   input.Role,
		Content:      content,
		CreatedAt:    ticket.CreatedAt,
	}

	err = dbstore.DB.Transaction(func(tx *gorm.DB) error {
		if err := ticketstore.CreateTicketTx(tx, ticket); err != nil {
			return err
		}
		entry.TicketId = ticket.Id
		if err := ticketstore.CreateTicketEntryTx(tx, entry); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, errors.New(i18n.Translate(lang, i18n.MsgTicketCreateFailed))
	}

	return buildTicketDetail(ticket, []*ticketstore.TicketEntry{entry}), nil
}

func GetTicketDetail(lang string, ticketId int, userId int, role int) (*TicketDetail, error) {
	ticket, err := ticketstore.GetTicketByID(ticketId)
	if err != nil {
		return nil, err
	}
	if err := ensureTicketAccess(lang, ticket, userId, role); err != nil {
		return nil, err
	}
	entries, err := ticketstore.GetTicketEntries(ticketId)
	if err != nil {
		return nil, err
	}
	return buildTicketDetail(ticket, entries), nil
}

func ReplyTicket(lang string, input ReplyTicketInput) error {
	content, err := validateTicketText(lang, input.Content, maxTicketContentRunes, i18n.MsgTicketReplyContentEmpty, i18n.MsgTicketReplyTooLong)
	if err != nil {
		return err
	}

	return dbstore.DB.Transaction(func(tx *gorm.DB) error {
		ticket, err := ticketstore.GetTicketByIDForUpdate(tx, input.TicketId)
		if err != nil {
			return err
		}
		if err := ensureTicketAccess(lang, ticket, input.UserId, input.Role); err != nil {
			return err
		}

		now := common.GetTimestamp()
		entry := &ticketstore.TicketEntry{
			TicketId:     ticket.Id,
			EntryType:    ticketstore.TicketEntryTypeMessage,
			SenderUserId: input.UserId,
			SenderName:   input.Username,
			SenderRole:   input.Role,
			Content:      content,
			CreatedAt:    now,
		}
		if err := ticketstore.CreateTicketEntryTx(tx, entry); err != nil {
			return errors.New(i18n.Translate(lang, i18n.MsgTicketReplyFailed))
		}
		if err := ticketstore.UpdateTicketFieldsTx(tx, ticket.Id, map[string]any{"updated_at": now}); err != nil {
			return errors.New(i18n.Translate(lang, i18n.MsgTicketReplyFailed))
		}
		return nil
	})
}

func CloseTicket(lang string, ticketId int, userId int, role int, username string) error {
	return changeTicketStatus(lang, ticketId, userId, role, username, ticketstore.TicketStatusCompleted)
}

func UpdateTicketStatus(lang string, ticketId int, userId int, role int, username string, status string) error {
	if !canManageAllTickets(role) {
		return errors.New(i18n.Translate(lang, i18n.MsgTicketNoPermission))
	}
	targetStatus, err := ticketstore.ParseTicketStatus(status)
	if err != nil {
		return err
	}
	return changeTicketStatus(lang, ticketId, userId, role, username, targetStatus)
}

func changeTicketStatus(lang string, ticketId int, userId int, role int, username string, targetStatus int) error {
	return dbstore.DB.Transaction(func(tx *gorm.DB) error {
		ticket, err := ticketstore.GetTicketByIDForUpdate(tx, ticketId)
		if err != nil {
			return err
		}
		if err := ensureTicketAccess(lang, ticket, userId, role); err != nil {
			return err
		}
		if ticket.Status == targetStatus {
			return nil
		}

		now := common.GetTimestamp()
		values := map[string]any{
			"status":     targetStatus,
			"updated_at": now,
			"closed_at":  int64(0),
		}
		if targetStatus == ticketstore.TicketStatusCompleted {
			values["closed_at"] = now
		}
		updated, err := ticketstore.UpdateTicketStatusTx(tx, ticket.Id, ticket.Status, values)
		if err != nil {
			return errors.New(i18n.Translate(lang, i18n.MsgTicketStatusUpdateFailed))
		}
		if !updated {
			return errors.New(i18n.Translate(lang, i18n.MsgTicketStatusConflict))
		}

		entry := &ticketstore.TicketEntry{
			TicketId:     ticket.Id,
			EntryType:    ticketstore.TicketEntryTypeStatusChange,
			SenderUserId: userId,
			SenderName:   username,
			SenderRole:   role,
			FromStatus:   ticket.Status,
			ToStatus:     targetStatus,
			CreatedAt:    now,
		}
		if err := ticketstore.CreateTicketEntryTx(tx, entry); err != nil {
			return errors.New(i18n.Translate(lang, i18n.MsgTicketStatusUpdateFailed))
		}
		return nil
	})
}

func buildTicketListFilter(userId int, page int, pageSize int, status string, keyword string) (ticketstore.TicketListFilter, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = common.ItemsPerPage
	}
	if pageSize > 100 {
		pageSize = 100
	}

	filter := ticketstore.TicketListFilter{
		UserId:  userId,
		Keyword: strings.TrimSpace(keyword),
		Offset:  (page - 1) * pageSize,
		Limit:   pageSize,
	}
	if status == "" || status == "all" {
		return filter, nil
	}
	parsedStatus, err := ticketstore.ParseTicketStatus(status)
	if err != nil {
		return ticketstore.TicketListFilter{}, err
	}
	filter.Status = parsedStatus
	return filter, nil
}

func buildTicketSummaries(tickets []*ticketstore.Ticket) []TicketSummary {
	items := make([]TicketSummary, 0, len(tickets))
	for _, ticket := range tickets {
		items = append(items, TicketSummary{
			Id:        ticket.Id,
			Title:     ticket.Title,
			Type:      ticketstore.TicketTypeName(ticket.Type),
			Status:    ticketstore.TicketStatusName(ticket.Status),
			CreatedAt: ticket.CreatedAt,
			UpdatedAt: ticket.UpdatedAt,
		})
	}
	return items
}

func buildTicketDetail(ticket *ticketstore.Ticket, entries []*ticketstore.TicketEntry) *TicketDetail {
	messages := make([]TicketMessage, 0, len(entries))
	for _, entry := range entries {
		message := TicketMessage{
			Id:       entry.Id,
			Username: entry.SenderName,
			Role:     buildMessageRole(entry.SenderRole),
			Time:     entry.CreatedAt,
		}
		if entry.EntryType == ticketstore.TicketEntryTypeStatusChange {
			message.Type = "status"
			message.Value = ticketstore.TicketStatusName(entry.ToStatus)
		} else {
			message.Type = "message"
			message.Content = entry.Content
		}
		messages = append(messages, message)
	}

	return &TicketDetail{
		Id:        ticket.Id,
		Title:     ticket.Title,
		Type:      ticketstore.TicketTypeName(ticket.Type),
		Status:    ticketstore.TicketStatusName(ticket.Status),
		CreatedAt: ticket.CreatedAt,
		UpdatedAt: ticket.UpdatedAt,
		ClosedAt:  ticket.ClosedAt,
		Messages:  messages,
	}
}

func buildMessageRole(role int) string {
	if canManageAllTickets(role) {
		return "admin"
	}
	return "user"
}

func ensureTicketAccess(lang string, ticket *ticketstore.Ticket, userId int, role int) error {
	if canManageAllTickets(role) {
		return nil
	}
	if ticket.UserId != userId {
		return errors.New(i18n.Translate(lang, i18n.MsgTicketAccessDenied))
	}
	return nil
}

func canManageAllTickets(role int) bool {
	return role >= common.RoleAdminUser
}

func validateTicketText(lang string, value string, maxRunes int, emptyMessageKey string, tooLongMessageKey string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New(i18n.Translate(lang, emptyMessageKey))
	}
	if utf8.RuneCountInString(value) > maxRunes {
		return "", errors.New(i18n.Translate(lang, tooLongMessageKey))
	}
	return value, nil
}
