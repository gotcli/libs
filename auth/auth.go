package auth

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

const (
	AccessTokenType  = "access"
	RefreshTokenType = "refresh"
	ClaimsLocal      = "jwt_claims"
)

type Claims struct {
	UserID      int64    `json:"user_id"`
	WorkspaceID int64    `json:"workspace_id,omitempty"`
	Roles       []string `json:"roles,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
	TokenType   string   `json:"token_type"`
	jwt.RegisteredClaims
}

type Verifier struct {
	issuer   string
	audience string
	secret   []byte
}

type claimsContextKey struct{}

func NewVerifier(issuer, audience, secret string) (*Verifier, error) {
	if issuer == "" || audience == "" {
		return nil, errors.New("JWT issuer and audience are required")
	}
	if len(secret) < 32 {
		return nil, errors.New("JWT access secret must contain at least 32 bytes")
	}
	return &Verifier{issuer: issuer, audience: audience, secret: []byte(secret)}, nil
}

func (v *Verifier) ParseAccess(raw string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected JWT signing method %q", token.Method.Alg())
		}
		return v.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithIssuer(v.issuer),
		jwt.WithAudience(v.audience), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
	if err != nil || !token.Valid || claims.TokenType != AccessTokenType || claims.UserID <= 0 {
		return nil, errors.New("invalid or expired access token")
	}
	return claims, nil
}

func RequireJWT(verifier *Verifier) fiber.Handler {
	return func(c *fiber.Ctx) error {
		parts := strings.Fields(c.Get(fiber.HeaderAuthorization))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return fiber.NewError(fiber.StatusUnauthorized, "missing or invalid bearer token")
		}
		claims, err := verifier.ParseAccess(parts[1])
		if err != nil {
			return fiber.NewError(fiber.StatusUnauthorized, "invalid or expired access token")
		}
		if claims.WorkspaceID <= 0 && !claims.HasRole("platform_admin") {
			return fiber.NewError(fiber.StatusForbidden, "workspace context is required")
		}
		c.Locals(ClaimsLocal, claims)
		c.SetUserContext(context.WithValue(c.UserContext(), claimsContextKey{}, claims))
		return c.Next()
	}
}

func ClaimsFromContext(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(claimsContextKey{}).(*Claims)
	return claims, ok
}

func RegisterWorkspaceScope(db *gorm.DB) error {
	scope := func(tx *gorm.DB) {
		claims, ok := ClaimsFromContext(tx.Statement.Context)
		if !ok || claims.WorkspaceID <= 0 || claims.HasRole("platform_admin") || tx.Statement.Schema == nil {
			return
		}
		if tx.Statement.Schema.LookUpField("WorkspaceID") == nil {
			return
		}
		tx.Statement.AddClause(clause.Where{Exprs: []clause.Expression{clause.Eq{
			Column: clause.Column{Table: clause.CurrentTable, Name: "workspace_id"}, Value: claims.WorkspaceID,
		}}})
	}
	if err := db.Callback().Query().Before("gorm:query").Register("lineoa:workspace_query", scope); err != nil {
		return err
	}
	if err := db.Callback().Update().Before("gorm:update").Register("lineoa:workspace_update", scope); err != nil {
		return err
	}
	if err := db.Callback().Delete().Before("gorm:delete").Register("lineoa:workspace_delete", scope); err != nil {
		return err
	}
	return db.Callback().Create().Before("gorm:create").Register("lineoa:workspace_create", func(tx *gorm.DB) {
		claims, ok := ClaimsFromContext(tx.Statement.Context)
		if !ok || claims.WorkspaceID <= 0 || claims.HasRole("platform_admin") || tx.Statement.Schema == nil {
			return
		}
		field := tx.Statement.Schema.LookUpField("WorkspaceID")
		if field == nil {
			return
		}
		value := reflect.Indirect(tx.Statement.ReflectValue)
		if value.Kind() == reflect.Slice || value.Kind() == reflect.Array {
			for index := 0; index < value.Len(); index++ {
				setWorkspaceField(tx, field, value.Index(index), claims.WorkspaceID)
			}
			return
		}
		setWorkspaceField(tx, field, value, claims.WorkspaceID)
	})
}

func setWorkspaceField(tx *gorm.DB, field *schema.Field, value reflect.Value, workspaceID int64) {
	current, zero := field.ValueOf(tx.Statement.Context, value)
	if !zero && !workspaceValueMatches(current, workspaceID) {
		tx.AddError(errors.New("workspace_id does not match authenticated workspace"))
		return
	}
	if err := field.Set(tx.Statement.Context, value, workspaceID); err != nil {
		tx.AddError(err)
	}
}

func workspaceValueMatches(current any, workspaceID int64) bool {
	value := reflect.ValueOf(current)
	for value.IsValid() && (value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface) {
		if value.IsNil() {
			return false
		}
		value = value.Elem()
	}
	if !value.IsValid() {
		return false
	}
	switch value.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return value.Int() == workspaceID
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return value.Uint() == uint64(workspaceID)
	default:
		return false
	}
}

func ClaimsFrom(c *fiber.Ctx) (*Claims, bool) {
	claims, ok := c.Locals(ClaimsLocal).(*Claims)
	return claims, ok
}

// WorkspaceClaimsFrom returns authenticated claims with a valid workspace context.
func WorkspaceClaimsFrom(c *fiber.Ctx) (*Claims, error) {
	claims, ok := ClaimsFrom(c)
	if !ok || claims.WorkspaceID <= 0 {
		return nil, fiber.ErrForbidden
	}
	return claims, nil
}

func (c *Claims) HasRole(role string) bool {
	for _, current := range c.Roles {
		if current == role {
			return true
		}
	}
	return false
}

func (c *Claims) HasPermission(permission string) bool {
	for _, current := range c.Permissions {
		if current == permission {
			return true
		}
	}
	return false
}
