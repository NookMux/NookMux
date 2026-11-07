package passkeystore

import (
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	infradb "github.com/NookMux/NookMux/internal/infra/db"
	"github.com/NookMux/NookMux/internal/store/db"
	"github.com/NookMux/NookMux/pkg/jsonx"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"gorm.io/gorm"
	"strings"
	"time"
)

var (
	ErrPasskeyNotFound         = errors.New("passkey credential not found")
	ErrFriendlyPasskeyNotFound = errors.New("Passkey 验证失败，请重试或联系管理员")
)

type PasskeyCredential struct {
	ID              int            `json:"id" gorm:"primaryKey"`
	UserID          int            `json:"user_id" gorm:"index;not null"`
	CredentialID    string         `json:"credential_id" gorm:"type:varchar(512);uniqueIndex;not null"` // base64 encoded
	PublicKey       string         `json:"public_key" gorm:"type:text;not null"`                        // base64 encoded
	DeviceName      string         `json:"device_name" gorm:"type:varchar(255)"`
	AttestationType string         `json:"attestation_type" gorm:"type:varchar(255)"`
	AAGUID          string         `json:"aaguid" gorm:"type:varchar(512)"` // base64 encoded
	SignCount       uint32         `json:"sign_count" gorm:"default:0"`
	CloneWarning    bool           `json:"clone_warning"`
	UserPresent     bool           `json:"user_present"`
	UserVerified    bool           `json:"user_verified"`
	BackupEligible  bool           `json:"backup_eligible"`
	BackupState     bool           `json:"backup_state"`
	Transports      string         `json:"transports" gorm:"type:text"`
	Attachment      string         `json:"attachment" gorm:"type:varchar(32)"`
	LastUsedAt      *time.Time     `json:"last_used_at"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `json:"-" gorm:"index"`
}

func (p *PasskeyCredential) TransportList() []protocol.AuthenticatorTransport {
	if p == nil || strings.TrimSpace(p.Transports) == "" {
		return nil
	}
	var transports []string
	if err := jsonx.Unmarshal([]byte(p.Transports), &transports); err != nil {
		return nil
	}
	result := make([]protocol.AuthenticatorTransport, 0, len(transports))
	for _, transport := range transports {
		result = append(result, protocol.AuthenticatorTransport(transport))
	}
	return result
}

func (p *PasskeyCredential) SetTransports(list []protocol.AuthenticatorTransport) {
	if len(list) == 0 {
		p.Transports = ""
		return
	}
	stringList := make([]string, len(list))
	for i, transport := range list {
		stringList[i] = string(transport)
	}
	encoded, err := jsonx.Marshal(stringList)
	if err != nil {
		return
	}
	p.Transports = string(encoded)
}

func (p *PasskeyCredential) ToWebAuthnCredential() webauthn.Credential {
	flags := webauthn.CredentialFlags{
		UserPresent:    p.UserPresent,
		UserVerified:   p.UserVerified,
		BackupEligible: p.BackupEligible,
		BackupState:    p.BackupState,
	}

	credID, _ := base64.StdEncoding.DecodeString(p.CredentialID)
	pubKey, _ := base64.StdEncoding.DecodeString(p.PublicKey)
	aaguid, _ := base64.StdEncoding.DecodeString(p.AAGUID)

	return webauthn.Credential{
		ID:              credID,
		PublicKey:       pubKey,
		AttestationType: p.AttestationType,
		Transport:       p.TransportList(),
		Flags:           flags,
		Authenticator: webauthn.Authenticator{
			AAGUID:       aaguid,
			SignCount:    p.SignCount,
			CloneWarning: p.CloneWarning,
			Attachment:   protocol.AuthenticatorAttachment(p.Attachment),
		},
	}
}

func NewPasskeyCredentialFromWebAuthn(userID int, credential *webauthn.Credential) *PasskeyCredential {
	if credential == nil {
		return nil
	}
	passkey := &PasskeyCredential{
		UserID:          userID,
		CredentialID:    base64.StdEncoding.EncodeToString(credential.ID),
		PublicKey:       base64.StdEncoding.EncodeToString(credential.PublicKey),
		AttestationType: credential.AttestationType,
		AAGUID:          base64.StdEncoding.EncodeToString(credential.Authenticator.AAGUID),
		SignCount:       credential.Authenticator.SignCount,
		CloneWarning:    credential.Authenticator.CloneWarning,
		UserPresent:     credential.Flags.UserPresent,
		UserVerified:    credential.Flags.UserVerified,
		BackupEligible:  credential.Flags.BackupEligible,
		BackupState:     credential.Flags.BackupState,
		Attachment:      string(credential.Authenticator.Attachment),
	}
	passkey.SetTransports(credential.Transport)
	return passkey
}

func (p *PasskeyCredential) ApplyValidatedCredential(credential *webauthn.Credential) {
	if credential == nil || p == nil {
		return
	}
	p.CredentialID = base64.StdEncoding.EncodeToString(credential.ID)
	p.PublicKey = base64.StdEncoding.EncodeToString(credential.PublicKey)
	p.AttestationType = credential.AttestationType
	p.AAGUID = base64.StdEncoding.EncodeToString(credential.Authenticator.AAGUID)
	p.SignCount = credential.Authenticator.SignCount
	p.CloneWarning = credential.Authenticator.CloneWarning
	p.UserPresent = credential.Flags.UserPresent
	p.UserVerified = credential.Flags.UserVerified
	p.BackupEligible = credential.Flags.BackupEligible
	p.BackupState = credential.Flags.BackupState
	p.Attachment = string(credential.Authenticator.Attachment)
	p.SetTransports(credential.Transport)
}

func GetPasskeyByUserID(userID int) (*PasskeyCredential, error) {
	if userID == 0 {
		common.SysLog("GetPasskeyByUserID: empty user ID")
		return nil, ErrFriendlyPasskeyNotFound
	}
	var credential PasskeyCredential
	if err := dbstore.DB.Where("user_id = ?", userID).First(&credential).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPasskeyNotFound
		}
		common.SysLog(fmt.Sprintf("GetPasskeyByUserID: database error for user %d: %v", userID, err))
		return nil, ErrFriendlyPasskeyNotFound
	}
	return &credential, nil
}

func GetPasskeysByUserID(userID int) ([]*PasskeyCredential, error) {
	if userID == 0 {
		return nil, fmt.Errorf("无效的用户 ID")
	}
	var credentials []*PasskeyCredential
	query := dbstore.DB.Where("user_id = ?", userID)
	if infradb.UsingMySQL {
		query = query.Order("IFNULL(last_used_at, '1970-01-01') DESC, created_at DESC")
	} else {
		query = query.Order("last_used_at DESC NULLS LAST, created_at DESC")
	}
	if err := query.Find(&credentials).Error; err != nil {
		common.SysLog(fmt.Sprintf("GetPasskeysByUserID: database error for user %d: %v", userID, err))
		return nil, fmt.Errorf("获取 Passkey 列表失败")
	}
	return credentials, nil
}

func GetPasskeyByID(id int) (*PasskeyCredential, error) {
	if id == 0 {
		return nil, fmt.Errorf("无效的 Passkey ID")
	}
	var credential PasskeyCredential
	if err := dbstore.DB.Where("id = ?", id).First(&credential).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("Passkey 不存在")
		}
		common.SysLog(fmt.Sprintf("GetPasskeyByID: database error for id %d: %v", id, err))
		return nil, fmt.Errorf("获取 Passkey 失败")
	}
	return &credential, nil
}

func CountPasskeysByUserID(userID int) (int64, error) {
	if userID == 0 {
		return 0, fmt.Errorf("无效的用户 ID")
	}
	var count int64
	if err := dbstore.DB.Model(&PasskeyCredential{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		common.SysLog(fmt.Sprintf("CountPasskeysByUserID: database error for user %d: %v", userID, err))
		return 0, fmt.Errorf("获取 Passkey 数量失败")
	}
	return count, nil
}

func GetPasskeyByCredentialID(credentialID []byte) (*PasskeyCredential, error) {
	if len(credentialID) == 0 {
		common.SysLog("GetPasskeyByCredentialID: empty credential ID")
		return nil, ErrFriendlyPasskeyNotFound
	}

	credIDStr := base64.StdEncoding.EncodeToString(credentialID)
	var credential PasskeyCredential
	if err := dbstore.DB.Where("credential_id = ?", credIDStr).First(&credential).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.SysLog(fmt.Sprintf("GetPasskeyByCredentialID: passkey not found for credential ID length %d", len(credentialID)))
			return nil, ErrFriendlyPasskeyNotFound
		}
		common.SysLog(fmt.Sprintf("GetPasskeyByCredentialID: database error for credential ID: %v", err))
		return nil, ErrFriendlyPasskeyNotFound
	}

	return &credential, nil
}

func CreatePasskeyCredential(credential *PasskeyCredential) error {
	if credential == nil {
		common.SysLog("CreatePasskeyCredential: nil credential provided")
		return fmt.Errorf("Passkey 保存失败，请重试")
	}
	if err := dbstore.DB.Create(credential).Error; err != nil {
		common.SysLog(fmt.Sprintf("CreatePasskeyCredential: failed to create credential for user %d: %v", credential.UserID, err))
		return fmt.Errorf("Passkey 保存失败，请重试")
	}
	return nil
}

func UpdatePasskeyCredential(credential *PasskeyCredential) error {
	if credential == nil || credential.ID == 0 {
		common.SysLog("UpdatePasskeyCredential: invalid credential")
		return fmt.Errorf("Passkey 更新失败，请重试")
	}
	if err := dbstore.DB.Save(credential).Error; err != nil {
		common.SysLog(fmt.Sprintf("UpdatePasskeyCredential: failed to update credential %d: %v", credential.ID, err))
		return fmt.Errorf("Passkey 更新失败，请重试")
	}
	return nil
}

func UpsertPasskeyCredential(credential *PasskeyCredential) error {
	if credential == nil {
		common.SysLog("UpsertPasskeyCredential: nil credential provided")
		return fmt.Errorf("Passkey 保存失败，请重试")
	}
	return dbstore.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("user_id = ?", credential.UserID).Delete(&PasskeyCredential{}).Error; err != nil {
			common.SysLog(fmt.Sprintf("UpsertPasskeyCredential: failed to delete existing credential for user %d: %v", credential.UserID, err))
			return fmt.Errorf("Passkey 保存失败，请重试")
		}
		if err := tx.Create(credential).Error; err != nil {
			common.SysLog(fmt.Sprintf("UpsertPasskeyCredential: failed to create credential for user %d: %v", credential.UserID, err))
			return fmt.Errorf("Passkey 保存失败，请重试")
		}
		return nil
	})
}

func DeletePasskeyByID(id int) error {
	if id == 0 {
		common.SysLog("DeletePasskeyByID: empty id")
		return fmt.Errorf("删除失败，请重试")
	}
	if err := dbstore.DB.Unscoped().Where("id = ?", id).Delete(&PasskeyCredential{}).Error; err != nil {
		common.SysLog(fmt.Sprintf("DeletePasskeyByID: failed to delete passkey %d: %v", id, err))
		return fmt.Errorf("删除失败，请重试")
	}
	return nil
}

func DeletePasskeyByUserID(userID int) error {
	if userID == 0 {
		common.SysLog("DeletePasskeyByUserID: empty user ID")
		return fmt.Errorf("删除失败，请重试")
	}
	if err := dbstore.DB.Unscoped().Where("user_id = ?", userID).Delete(&PasskeyCredential{}).Error; err != nil {
		common.SysLog(fmt.Sprintf("DeletePasskeyByUserID: failed to delete passkey for user %d: %v", userID, err))
		return fmt.Errorf("删除失败，请重试")
	}
	return nil
}

// DeletePasskeyByUserIDTx 在事务内硬删除用户全部 passkey 凭据，
// 供用户硬删除流程编排调用。
func DeletePasskeyByUserIDTx(tx *gorm.DB, userID int) error {
	if err := tx.Unscoped().Where("user_id = ?", userID).Delete(&PasskeyCredential{}).Error; err != nil {
		common.SysLog(fmt.Sprintf("DeletePasskeyByUserIDTx: failed for user %d: %v", userID, err))
		return fmt.Errorf("删除失败，请重试")
	}
	return nil
}
