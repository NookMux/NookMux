package ticketcontroller

import (
	"errors"
	"strconv"

	"github.com/NookMux/NookMux/internal/common"
	domainticket "github.com/NookMux/NookMux/internal/domain/ticket"
	"github.com/NookMux/NookMux/internal/httpapi"
	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/gin-gonic/gin"
)

func respondTicketError(c *gin.Context, logAction string, err error) {
	var localizedErr *i18n.Error
	if errors.As(err, &localizedErr) {
		httpapi.ApiErrorI18n(c, localizedErr.Key, localizedErr.Params)
		return
	}
	common.SysError(logAction + ": " + err.Error())
	httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
}

type createTicketRequest struct {
	Title   string `json:"title"`
	Type    string `json:"type"`
	Content string `json:"content"`
}

type replyTicketRequest struct {
	Content string `json:"content"`
}

type updateTicketStatusRequest struct {
	Status string `json:"status"`
}

func GetUserTickets(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	items, total, err := domainticket.ListUserTickets(c.GetInt("id"), pageInfo.GetPage(), pageInfo.GetPageSize(), c.DefaultQuery("status", "all"), c.Query("keyword"))
	if err != nil {
		respondTicketError(c, "failed to list user tickets", err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	httpapi.ApiSuccess(c, pageInfo)
}

func GetAdminTickets(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	items, total, err := domainticket.ListAdminTickets(i18n.GetLangFromContext(c), c.GetInt("role"), pageInfo.GetPage(), pageInfo.GetPageSize(), c.DefaultQuery("status", "all"), c.Query("keyword"))
	if err != nil {
		respondTicketError(c, "failed to list admin tickets", err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	httpapi.ApiSuccess(c, pageInfo)
}

func CreateTicket(c *gin.Context) {
	var req createTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidRequestBody)
		return
	}

	data, err := domainticket.CreateTicket(i18n.GetLangFromContext(c), domainticket.CreateTicketInput{
		UserId:   c.GetInt("id"),
		Username: c.GetString("username"),
		Role:     c.GetInt("role"),
		Title:    req.Title,
		Type:     req.Type,
		Content:  req.Content,
	})
	if err != nil {
		respondTicketError(c, "failed to create ticket", err)
		return
	}
	httpapi.ApiSuccess(c, data)
}

func GetTicketDetail(c *gin.Context) {
	ticketId, err := strconv.Atoi(c.Param("id"))
	if err != nil || ticketId <= 0 {
		httpapi.ApiErrorI18n(c, i18n.MsgTicketInvalidID)
		return
	}

	data, err := domainticket.GetTicketDetail(i18n.GetLangFromContext(c), ticketId, c.GetInt("id"), c.GetInt("role"))
	if err != nil {
		respondTicketError(c, "failed to get ticket detail", err)
		return
	}
	httpapi.ApiSuccess(c, data)
}

func ReplyTicket(c *gin.Context) {
	ticketId, err := strconv.Atoi(c.Param("id"))
	if err != nil || ticketId <= 0 {
		httpapi.ApiErrorI18n(c, i18n.MsgTicketInvalidID)
		return
	}

	var req replyTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidRequestBody)
		return
	}

	err = domainticket.ReplyTicket(i18n.GetLangFromContext(c), domainticket.ReplyTicketInput{
		TicketId: ticketId,
		UserId:   c.GetInt("id"),
		Username: c.GetString("username"),
		Role:     c.GetInt("role"),
		Content:  req.Content,
	})
	if err != nil {
		respondTicketError(c, "failed to reply ticket", err)
		return
	}
	httpapi.ApiSuccess(c, nil)
}

func CloseTicket(c *gin.Context) {
	ticketId, err := strconv.Atoi(c.Param("id"))
	if err != nil || ticketId <= 0 {
		httpapi.ApiErrorI18n(c, i18n.MsgTicketInvalidID)
		return
	}

	err = domainticket.CloseTicket(i18n.GetLangFromContext(c), ticketId, c.GetInt("id"), c.GetInt("role"), c.GetString("username"))
	if err != nil {
		respondTicketError(c, "failed to close ticket", err)
		return
	}
	httpapi.ApiSuccess(c, nil)
}

func UpdateTicketStatus(c *gin.Context) {
	ticketId, err := strconv.Atoi(c.Param("id"))
	if err != nil || ticketId <= 0 {
		httpapi.ApiErrorI18n(c, i18n.MsgTicketInvalidID)
		return
	}

	var req updateTicketStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.ApiErrorI18n(c, i18n.MsgInvalidRequestBody)
		return
	}

	err = domainticket.UpdateTicketStatus(i18n.GetLangFromContext(c), ticketId, c.GetInt("id"), c.GetInt("role"), c.GetString("username"), req.Status)
	if err != nil {
		respondTicketError(c, "failed to update ticket status", err)
		return
	}
	httpapi.ApiSuccess(c, nil)
}
