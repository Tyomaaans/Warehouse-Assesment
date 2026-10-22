package user

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"warehouse/internal/auth/domain"
	"warehouse/internal/auth/token"
	"warehouse/pkg"
)

type UserAuthHandler struct {
	userSvc       UserService
	secureCookies bool
}

func NewUserHandler(userSvc UserService, secureCookies bool) *UserAuthHandler {
	return &UserAuthHandler{userSvc: userSvc, secureCookies: secureCookies}
}

func (h *UserAuthHandler) LoginUser(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var req LoginRequest
	if err := pkg.BindJSON(c, &req); err != nil {
		pkg.HTTPError(c, err)
		return
	}
	response, pair, err := h.userSvc.LoginUser(c.Request.Context(), req)
	if err != nil {
		pkg.HTTPError(c, err)
		return
	}
	h.setCookies(c, pair)
	pkg.SuccessResponse(c, http.StatusOK, "", response)
}

func (h *UserAuthHandler) RefreshTokens(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	raw, sessionID, err := sessionCookies(c)
	if err != nil {
		pkg.HTTPError(c, err)
		return
	}
	pair, err := h.userSvc.RefreshTokens(c.Request.Context(), raw, sessionID)
	if err != nil {
		pkg.HTTPError(c, err)
		return
	}
	h.setCookies(c, pair)
	pkg.SuccessResponse(c, http.StatusOK, "", RefreshResponse{AccessToken: pair.AccessToken, ExpiresIn: int(token.AccessTokenExpiry.Seconds())})
}

func (h *UserAuthHandler) LogoutUser(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	raw, sessionID, err := sessionCookies(c)
	if err != nil {
		pkg.HTTPError(c, err)
		return
	}
	if err := h.userSvc.LogoutUser(c.Request.Context(), raw, sessionID); err != nil {
		pkg.HTTPError(c, err)
		return
	}
	h.setCookie(c, "refresh_token", "", "/api/v1/auth", -1)
	h.setCookie(c, "session_id", "", "/api/v1", -1)
	c.Status(http.StatusNoContent)
}

func sessionCookies(c *gin.Context) (string, uuid.UUID, error) {
	refresh, err := c.Cookie("refresh_token")
	if err != nil || refresh == "" {
		return "", uuid.Nil, pkg.ErrUnauthorized
	}
	sessionID, err := c.Cookie("session_id")
	if err != nil || sessionID == "" {
		return "", uuid.Nil, pkg.ErrUnauthorized
	}
	id, err := pkg.ParseID(sessionID)
	return refresh, id, err
}

func (h *UserAuthHandler) setCookies(c *gin.Context, pair *domain.TokenPairEntity) {
	maxAge := int(pair.TTL / time.Second)
	h.setCookie(c, "refresh_token", pair.RefreshToken, "/api/v1/auth", maxAge)
	h.setCookie(c, "session_id", pkg.EncodeID(uuid.MustParse(pair.SessionID)), "/api/v1", maxAge)
}

func (h *UserAuthHandler) setCookie(c *gin.Context, name, value, path string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: name, Value: value, Path: path, MaxAge: maxAge,
		HttpOnly: true, Secure: h.secureCookies, SameSite: http.SameSiteStrictMode,
	})
}
