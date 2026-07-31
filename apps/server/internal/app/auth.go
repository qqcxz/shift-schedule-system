package app

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type authClaims struct {
	Role               string `json:"role"`
	StoreID            string `json:"storeId"`
	Username           string `json:"username"`
	MustChangePassword bool   `json:"mustChangePassword,omitempty"`
	AuthVersion        int    `json:"authVersion,omitempty"`
	jwt.RegisteredClaims
}

type authContextKey struct{}

func (s *Server) signToken(user User) (string, error) {
	now := time.Now()
	claims := authClaims{
		Role:               user.Role,
		StoreID:            user.StoreID,
		Username:           user.Username,
		MustChangePassword: user.MustChangePassword,
		AuthVersion:        user.AuthVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(7 * 24 * time.Hour)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.cfg.JWTSecret))
}

func parseToken(secret, raw string) (authClaims, error) {
	claims := authClaims{}
	token, err := jwt.ParseWithClaims(raw, &claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, unauthorized("Unauthorized")
		}
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil || !token.Valid || claims.Subject == "" {
		return authClaims{}, unauthorized("Unauthorized")
	}
	return claims, nil
}

func bearerToken(header string) string {
	parts := strings.Fields(header)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return parts[1]
	}
	return ""
}

func (s *Server) authenticate(next apiHandler, roles ...string) apiHandler {
	return func(w http.ResponseWriter, r *http.Request) error {
		token := bearerToken(r.Header.Get("Authorization"))
		if token == "" {
			return unauthorized("Unauthorized")
		}
		claims, err := parseToken(s.cfg.JWTSecret, token)
		if err != nil {
			return err
		}
		claims, err = activeAuthClaims(r.Context(), s.db, claims)
		if err != nil {
			return err
		}
		if claims.MustChangePassword && r.URL.Path != "/api/auth/me" && r.URL.Path != "/api/auth/change-password" {
			return forbidden("首次登录请先修改密码")
		}
		if len(roles) > 0 {
			allowed := false
			for _, role := range roles {
				if claims.Role == role {
					allowed = true
					break
				}
			}
			if !allowed {
				return forbidden("没有权限执行该操作")
			}
		}
		return next(w, r.WithContext(context.WithValue(r.Context(), authContextKey{}, claims)))
	}
}

func requestClaims(r *http.Request) authClaims {
	claims, _ := r.Context().Value(authContextKey{}).(authClaims)
	return claims
}

func activeAuthClaims(ctx context.Context, runner dbRunner, claims authClaims) (authClaims, error) {
	user, err := scanUser(runner.QueryRowContext(ctx, "SELECT "+userColumns+" FROM `users` WHERE `id` = ? LIMIT 1", claims.Subject))
	if errors.Is(err, sql.ErrNoRows) {
		return authClaims{}, unauthorized("Unauthorized")
	}
	if err != nil {
		return authClaims{}, err
	}
	if !user.IsActive {
		return authClaims{}, unauthorized("Unauthorized")
	}
	if claims.AuthVersion != 0 && claims.AuthVersion != user.AuthVersion {
		return authClaims{}, unauthorized("Unauthorized")
	}
	if claims.AuthVersion == 0 && user.AuthVersion > 1 {
		return authClaims{}, unauthorized("Unauthorized")
	}
	claims.Role = user.Role
	claims.StoreID = user.StoreID
	claims.Username = user.Username
	claims.MustChangePassword = user.MustChangePassword
	claims.AuthVersion = user.AuthVersion
	return claims, nil
}

type loginInput struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) error {
	var input loginInput
	if err := decodeJSON(r, &input); err != nil {
		return err
	}
	input.Username = normalizeUsername(input.Username)
	if input.Username == "" || len([]rune(input.Password)) < 6 || len([]rune(input.Password)) > 64 {
		return badRequest("用户名和密码格式不正确")
	}
	user, err := findUserByUsername(r.Context(), s.db, input.Username)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !user.IsActive) {
		return unauthorized("用户名或密码错误")
	}
	if err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)) != nil {
		return unauthorized("用户名或密码错误")
	}
	token, err := s.signToken(user)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, map[string]any{"accessToken": token, "user": user})
	return nil
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) error {
	user, err := findUserByID(r.Context(), s.db, requestClaims(r).Subject)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, user)
	return nil
}

type changePasswordInput struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) error {
	var input changePasswordInput
	if err := decodeJSON(r, &input); err != nil {
		return err
	}
	if input.CurrentPassword == "" || len([]rune(input.NewPassword)) < 6 || len([]rune(input.NewPassword)) > 64 {
		return badRequest("密码格式不正确")
	}
	if input.CurrentPassword == input.NewPassword {
		return badRequest("新密码不能与当前密码相同")
	}
	user, err := findUserByID(r.Context(), s.db, requestClaims(r).Subject)
	if err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.CurrentPassword)) != nil {
		return badRequest("当前密码不正确")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), 10)
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(r.Context(), "UPDATE `users` SET `passwordHash` = ?, `mustChangePassword` = 0, `authVersion` = `authVersion` + 1, `updatedAt` = ? WHERE `id` = ?", string(hash), nowDB(), user.ID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return unauthorized("Unauthorized")
	}
	s.realtime.DisconnectUser(user.ID)
	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
	return nil
}
