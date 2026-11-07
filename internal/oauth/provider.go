package oauth

import (
	"context"
	"github.com/NookMux/NookMux/internal/store/user"
	"github.com/gin-gonic/gin"
)

// Provider defines the interface for OAuth providers
type Provider interface {
	// GetName returns the display name of the provider (e.g., "GitHub", "Discord")
	GetName() string

	// IsEnabled returns whether this OAuth provider is enabled
	IsEnabled() bool

	// ExchangeToken exchanges the authorization code for an access token
	// The gin.Context is passed for providers that need request info (e.g., for redirect_uri)
	ExchangeToken(ctx context.Context, code string, c *gin.Context) (*OAuthToken, error)

	// GetUserInfo retrieves user information using the access token
	GetUserInfo(ctx context.Context, token *OAuthToken) (*OAuthUser, error)

	// IsUserIDTaken checks if the provider user ID is already associated with an account
	IsUserIDTaken(providerUserID string) bool

	// FillUserByProviderID fills the user model by provider user ID
	FillUserByProviderID(user *userstore.User, providerUserID string) error

	// SetProviderUserID sets the provider user ID on the user model
	SetProviderUserID(user *userstore.User, providerUserID string)

	// BindColumn returns the users table column that stores this provider's
	// binding (e.g. "github_id"). Targeted single-column updates use it so a
	// bind never writes columns beyond its own binding column.
	BindColumn() string
}
