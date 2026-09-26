package ticketcontroller

import (
	"strconv"

	"github.com/NookMux/NookMux/internal/common"
	domainticket "github.com/NookMux/NookMux/internal/domain/ticket"
	"github.com/NookMux/NookMux/internal/httpapi"
	"github.com/NookMux/NookMux/internal/i18n"
	"github.com/gin-gonic/gin"
)

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
		common.SysError("failed to list user tickets: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
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
		common.SysError("failed to list admin tickets: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
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
		common.SysError("failed to create ticket: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
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
		common.SysError("failed to get ticket detail: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
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
		common.SysError("failed to reply ticket: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
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
		common.SysError("failed to close ticket: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
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
		common.SysError("failed to update ticket status: " + err.Error())
		httpapi.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	httpapi.ApiSuccess(c, nil)
}
